// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Variable frequency CPU usage sampling via /proc and delay stats via netlink

// easy
// TODO - tui use keyboard to highlight a proc to make it be on top
// TODO - tui use exited procs if they are still in the topN

// hard
// TODO - use netlink to watch for processes exiting, or perf_events for start/exit

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"sort"
	"time"

	lib "github.com/uber-common/cpustat/lib"
)

func checkPrivs() {
	if os.Geteuid() != 0 {
		fmt.Println("This program uses the netlink taskstats inteface, so it must be run as root.")
		os.Exit(1)
	}
}

func maybeStartProfile(argStr string) {
	if argStr != "" {
		f, err := os.Create(argStr)
		if err != nil {
			log.Fatal(err)
		}
		if err = pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
	}
}

func waitForExit(memprofile string) chan string {
	uiQuitChan := make(chan string)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		select {
		case <-sigChan:
			fmt.Fprintln(os.Stderr, "quitting on signal")
		case msg := <-uiQuitChan:
			fmt.Fprintln(os.Stderr, msg)
		}
		pprof.StopCPUProfile()

		if memprofile != "" {
			f, err := os.Create(memprofile)
			if err != nil {
				log.Fatal(err)
			}
			pprof.WriteHeapProfile(f)
			f.Close()
		}

		os.Exit(0)
	}()

	return uiQuitChan
}

func main() {
	var interval = flag.Int("i", 200, "interval (ms) between measurements")
	var samples = flag.Int("s", 10, "sample counts to aggregate for output")
	var topN = flag.Int("n", 10, "show top N processes")
	var maxProcsToScan = flag.Int("maxprocs", 2048, "upper limit on process table size")
	var usrOnly = flag.String("u", "", "only show procs owned by this list of users")
	var pidOnly = flag.String("p", "", "only show procs in this list of pids")
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	var memprofile = flag.String("memprofile", "", "write memory profile to this file")
	var jiffy = flag.Int("jiffy", 100, "length of a jiffy")
	var useTui = flag.Bool("t", false, "use fancy terminal mode")

	flag.Parse()

	checkPrivs()

	if *interval < 10 {
		fmt.Println("The minimum sampling interval is 10ms")
		os.Exit(1)
	}
	intervalms := uint32(*interval)

	maybeStartProfile(*cpuprofile)
	uiQuitChan := waitForExit(*memprofile)
	filters := lib.FiltersInit(*usrOnly, *pidOnly)
	nlConn := lib.NLInit()

	if *useTui {
		go tuiInit(uiQuitChan, *interval)
	} else {
		textInit(*interval, *samples, *topN, filters)
	}

	infoMap := make(lib.ProcInfoMap)

	procCur := lib.NewProcSampleList(*maxProcsToScan)
	procPrev := lib.NewProcSampleList(*maxProcsToScan)
	procSum := make(lib.ProcSampleMap)
	procHist := make(lib.ProcStatsHistMap)
	taskHist := make(lib.TaskStatsHistMap)

	var sysCur lib.SystemStats
	var sysPrev lib.SystemStats
	var sysSum *lib.SystemStats
	var sysHist *lib.SystemStatsHist

	var t1, t2 time.Time
	var err error

	// run all scans one time to establish a baseline
	pids := make(lib.Pidlist, 0, *maxProcsToScan)

	t1 = time.Now()
	lib.GetPidList(&pids, *maxProcsToScan)
	lib.ProcStatsReader(pids, filters, &procPrev, infoMap)
	lib.TaskStatsReader(nlConn, pids, &procPrev)
	err = lib.SystemStatsReader(&sysPrev)
	if err != nil {
		panic(err)
	}

	sysSum = &lib.SystemStats{}
	sysHist = lib.NewSysStatsHist()
	t2 = time.Now()

	targetSleep := time.Duration(*interval) * time.Millisecond
	adjustedSleep := targetSleep - t2.Sub(t1)

	topPids := make(lib.Pidlist, *topN)
	for {
		for count := 0; count < *samples; count++ {
			time.Sleep(adjustedSleep)

			t1 = time.Now()
			lib.GetPidList(&pids, *maxProcsToScan)

			lib.ProcStatsReader(pids, filters, &procCur, infoMap)
			lib.TaskStatsReader(nlConn, pids, &procCur)

			procDelta := make(lib.ProcSampleMap, len(pids))
			lib.ProcStatsRecord(intervalms, procCur, procPrev, procSum, procDelta)
			lib.UpdateProcStatsHist(procHist, procDelta)
			lib.TaskStatsRecord(intervalms, procCur, procPrev, procSum, procDelta)
			lib.UpdateTaskStatsHist(taskHist, procDelta)

			procPrev, procCur = procCur, procPrev

			if err = lib.SystemStatsReader(&sysCur); err != nil {
				log.Fatal(err)
			}
			sysDelta := lib.SystemStatsRecord(intervalms, &sysCur, &sysPrev, sysSum)
			lib.UpdateSysStatsHist(sysHist, sysDelta)
			sysPrev = sysCur

			if *useTui {
				tuiGraphUpdate(procDelta, sysDelta, topPids, uint32(*jiffy), intervalms)
			}

			t2 = time.Now()
			adjustedSleep = targetSleep - t2.Sub(t1)
		}

		topHist := sortList(procHist, taskHist, *topN)
		topPids = topPids[:len(topHist)]
		for i := 0; i < len(topHist) && i < *topN; i++ {
			topPids[i] = topHist[i].pid
		}

		if *useTui {
			tuiListUpdate(infoMap, topPids, procSum, procHist, taskHist, sysSum, sysHist, *jiffy, *interval, *samples)
		} else {
			dumpStats(infoMap, topPids, procSum, procHist, taskHist, sysSum, sysHist, *jiffy, *interval, *samples)
		}
		procHist = make(lib.ProcStatsHistMap)
		taskHist = make(lib.TaskStatsHistMap)
		procSum = make(lib.ProcSampleMap)
		sysHist = lib.NewSysStatsHist()
		sysSum = &lib.SystemStats{}
		t2 = time.Now()
		adjustedSleep = targetSleep - t2.Sub(t1)
		// If we can't keep up, try to buy ourselves a little headroom by sleeping for a magic number of ms
		if adjustedSleep <= 0 {
			adjustedSleep = time.Duration(100) * time.Millisecond
		}
	}
}

// Wrapper to sort histograms by max but remember which pid they are
type sortHist struct {
	pid  int
	proc *lib.ProcStatsHist
	task *lib.TaskStatsHist
}

// ByMax sorts stats by max usage
type ByMax []*sortHist

func (m ByMax) Len() int {
	return len(m)
}
func (m ByMax) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}
func (m ByMax) Less(i, j int) bool {
	var maxI, maxJ float64

	// We might have proc stats but no taskstats because of unfortuante timing
	if m[i].task == nil || m[j].task == nil {
		maxI = maxList([]float64{
			float64(m[i].proc.Ustime.Max()),
			float64(m[i].proc.Cutime.Max()+m[i].proc.Cstime.Max()) / 1000,
		})
		maxJ = maxList([]float64{
			float64(m[j].proc.Ustime.Max()),
			float64(m[j].proc.Cutime.Max()+m[j].proc.Cstime.Max()) / 1000,
		})
	} else {
		maxI = maxList([]float64{
			float64(m[i].proc.Ustime.Max()),
			float64(m[i].proc.Cutime.Max()+m[i].proc.Cstime.Max()) / 100,
			float64(m[i].task.Cpudelay.Max()) / 1000 / 1000,
			float64(m[i].task.Iowait.Max()) / 1000 / 1000,
			float64(m[i].task.Swap.Max()) / 1000 / 1000,
		})
		maxJ = maxList([]float64{
			float64(m[j].proc.Ustime.Max()),
			float64(m[j].proc.Cutime.Max()+m[j].proc.Cstime.Max()) / 100,
			float64(m[j].task.Cpudelay.Max()) / 1000 / 1000,
			float64(m[j].task.Iowait.Max()) / 1000 / 1000,
			float64(m[j].task.Swap.Max()) / 1000 / 1000,
		})
	}
	return maxI > maxJ
}
func maxList(list []float64) float64 {
	ret := list[0]
	for i := 1; i < len(list); i++ {
		if list[i] > ret {
			ret = list[i]
		}
	}
	return ret
}

func sortList(procHist lib.ProcStatsHistMap, taskHist lib.TaskStatsHistMap, limit int) []*sortHist {
	var list []*sortHist

	// let's hope that pid is in both sets, otherwise this will blow up
	for pid, hist := range procHist {
		list = append(list, &sortHist{pid, hist, taskHist[pid]})
	}
	sort.Sort(ByMax(list))

	if len(list) > limit {
		list = list[:limit]
	}

	return list
}
