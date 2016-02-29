//
// Variable frequency CPU usage sampling on Linux via /proc
// Maybe this will turn into something like prstat on Solaris

// easy
// TODO - tui better colors
// TODO - use the actual time of each measurement not the expected time
// TODO - tui use keyboard to highlight a proc to make it be on top
// TODO - tui use exited procs if they are still in the topN

// hard
// TODO - split into long running backend and multiple frontends

// check /proc/pid/stats periodically for thread count and child process exit time

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
	var interval = flag.Int("i", 1000, "interval (ms) between measurements")
	var samples = flag.Int("s", 60, "sample counts to aggregate for output")
	var topN = flag.Int("n", 10, "show top N processes")
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
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
		os.Exit(0)
	}()

	nlConn := NLInit()

	if *useTui {
		go tuiInit(uiQuitChan, *interval)
	}

	cmdNames := make(cmdlineMap)

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
	taskPrev = statReader(nlConn, pids, cmdNames)
	sysPrev = statReaderGlobal()
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

			taskCur = statReader(nlConn, pids, cmdNames)
			taskDelta := statRecord(*interval, taskCur, taskPrev, taskSum, taskHist)
			taskPrev = taskCur

			sysCur = statReaderGlobal()
			sysDelta := statsRecordGlobal(*interval, sysCur, sysPrev, sysSum, sysHist)
			sysPrev = sysCur

			if *useTui {
				tuiGraphUpdate(sysDelta, taskDelta, topPids, *jiffy, *interval)
			}

			t2 = time.Now()
			adjustedSleep = targetSleep - t2.Sub(t1)
		}

		topHist := sortList(taskHist, *topN)
		for i := 0; i < *topN; i++ {
			topPids[i] = topHist[i].pid
		}

		if *useTui {
			tuiListUpdate(cmdNames, topPids, taskSum, taskHist, sysSum, sysHist, jiffy, interval, samples)
		} else {
			dumpStats(cmdNames, topPids, taskSum, taskHist, sysSum, sysHist, jiffy, interval, samples)
		}
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
	hist *taskStatsHist
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
	maxI := maxList([]float64{
		float64(m[i].hist.ustime.Max()),
		float64(m[i].hist.iowait.Max()),
	})
	maxJ := maxList([]float64{
		float64(m[j].hist.ustime.Max()),
		float64(m[j].hist.iowait.Max()),
	})
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

func sortList(histStats taskStatsHistMap, limit int) []*sortHist {
	var list []*sortHist

	for pid, hist := range histStats {
		list = append(list, &sortHist{pid, hist})
	}
	sort.Sort(ByMax(list))

	if len(list) > limit {
		list = list[:limit]
	}

	return list
}
