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

// Variable frequency CPU usage sampling on Linux via /proc
// Maybe this will turn into something like prstat on Solaris

// easy
// TODO - tui better colors
// TODO - use the actual time of each measurement not the expected time
// TODO - tui use keyboard to highlight a proc to make it be on top
// TODO - tui use exited procs if they are still in the topN
// TODO - use netlink to watch for processes exiting

// hard
// TODO - use perf_events to watch for processes starting so we don't have to constantly scan
// TODO - split into long running backend and multiple frontends

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
)

const histMin = 0
const histMax = 100000000
const histSigFigs = 2
const maxProcsToScan = 2048

func main() {
	var interval = flag.Int("i", 200, "interval (ms) between measurements")
	var samples = flag.Int("s", 10, "sample counts to aggregate for output")
	var topN = flag.Int("n", 10, "show top N processes")
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	var memprofile = flag.String("memprofile", "", "write memory profile to this file")
	var jiffy = flag.Int("jiffy", 100, "length of a jiffy")
	var useTui = flag.Bool("t", false, "use fancy terminal mode")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		if err = pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
	}

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

		if *memprofile != "" {
			f, err := os.Create(*memprofile)
			if err != nil {
				log.Fatal(err)
			}
			pprof.WriteHeapProfile(f)
			f.Close()
		}
		os.Exit(0)
	}()

	nlConn := NLInit()

	if *useTui {
		go tuiInit(uiQuitChan, *interval)
	}

	cmdNames := make(cmdlineMap)

	procCur := make(procStatsMap)
	procPrev := make(procStatsMap)
	procSum := make(procStatsMap)
	procHist := make(procStatsHistMap)

	taskCur := make(taskStatsMap)
	taskPrev := make(taskStatsMap)
	taskSum := make(taskStatsMap)
	taskHist := make(taskStatsHistMap)

	var sysCur *systemStats
	var sysPrev *systemStats
	var sysSum *systemStats
	var sysHist *systemStatsHist

	var t1, t2 time.Time

	// run all scans one time to establish a baseline
	pids := make(pidlist, 0, maxProcsToScan)
	getPidList(&pids)

	t1 = time.Now()
	procPrev = procStatsReader(pids, cmdNames)
	taskPrev = taskStatsReader(nlConn, pids, cmdNames)
	sysPrev = systemStatsReader()
	sysSum = &systemStats{}
	sysHist = &systemStatsHist{}
	t2 = time.Now()

	targetSleep := time.Duration(*interval) * time.Millisecond
	adjustedSleep := targetSleep - t2.Sub(t1)

	topPids := make(pidlist, *topN)
	for {
		for count := 0; count < *samples; count++ {
			time.Sleep(adjustedSleep)

			t1 = time.Now()
			getPidList(&pids)

			procCur = procStatsReader(pids, cmdNames)
			procDelta := procStatsRecord(*interval, procCur, procPrev, procSum, procHist)
			procPrev = procCur

			taskCur = taskStatsReader(nlConn, pids, cmdNames)
			taskDelta := taskStatsRecord(*interval, taskCur, taskPrev, taskSum, taskHist)
			taskPrev = taskCur

			sysCur = systemStatsReader()
			sysDelta := systemStatsRecord(*interval, sysCur, sysPrev, sysSum, sysHist)
			sysPrev = sysCur

			if *useTui {
				tuiGraphUpdate(procDelta, sysDelta, taskDelta, topPids, *jiffy, *interval)
			}

			t2 = time.Now()
			adjustedSleep = targetSleep - t2.Sub(t1)
		}

		topHist := sortList(procHist, taskHist, *topN)
		for i := 0; i < *topN; i++ {
			topPids[i] = topHist[i].pid
		}

		if *useTui {
			tuiListUpdate(cmdNames, topPids, procSum, procHist, taskSum, taskHist, sysSum, sysHist, *jiffy, *interval, *samples)
		} else {
			dumpStats(cmdNames, topPids, procSum, procHist, taskSum, taskHist, sysSum, sysHist, *jiffy, *interval, *samples)
		}
		procHist = make(procStatsHistMap)
		procSum = make(procStatsMap)
		taskHist = make(taskStatsHistMap)
		taskSum = make(taskStatsMap)
		sysHist = &systemStatsHist{}
		sysSum = &systemStats{}
		t2 = time.Now()
		adjustedSleep = targetSleep - t2.Sub(t1)
	}
}

type bar struct {
	baz int
}

// Wrapper to sort histograms by max but remember which pid they are
type sortHist struct {
	pid  int
	proc *procStatsHist
	task *taskStatsHist
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

	if m[i].task == nil || m[j].task == nil {
		maxI = maxList([]float64{
			float64(m[i].proc.ustime.Max()),
			float64(m[i].proc.cutime.Max()+m[i].proc.cstime.Max()) / 1000,
		})
		maxJ = maxList([]float64{
			float64(m[j].proc.ustime.Max()),
			float64(m[j].proc.cutime.Max()+m[j].proc.cstime.Max()) / 1000,
		})
	} else {
		maxI = maxList([]float64{
			float64(m[i].proc.ustime.Max()),
			float64(m[i].proc.cutime.Max()+m[i].proc.cstime.Max()) / 100,
			float64(m[i].task.cpudelay.Max()) / 1000 / 1000,
			float64(m[i].task.iowait.Max()) / 1000 / 1000,
			float64(m[i].task.swap.Max()) / 1000 / 1000,
		})
		maxJ = maxList([]float64{
			float64(m[j].proc.ustime.Max()),
			float64(m[j].proc.cutime.Max()+m[j].proc.cstime.Max()) / 100,
			float64(m[j].task.cpudelay.Max()) / 1000 / 1000,
			float64(m[j].task.iowait.Max()) / 1000 / 1000,
			float64(m[j].task.swap.Max()) / 1000 / 1000,
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

func sortList(procHist procStatsHistMap, taskHist taskStatsHistMap, limit int) []*sortHist {
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
