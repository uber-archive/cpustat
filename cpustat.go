//
// Variable frequency CPU usage sampling on Linux via /proc
// Maybe this will turn into something like prstat on Solaris
//

// TOOD - check for rollover in sched
// TODO - /proc/stat parser to handle embedded parens
// TODO - check max field length for assumed present fields
// TODO - tui fill out text area
// TODO - tui y axis scale
// TODO - tui x axis labels
// TODO - tui consistent proc colors
// TODO - tui proc labels

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

	if *useTui {
		go tuiInit(uiQuitChan)
	}

	procCur := make(procStatsMap)
	procPrev := make(procStatsMap)
	procSum := make(procStatsMap)
	procHist := make(procStatsHistMap)

	schedCur := make(schedStatsMap)
	schedPrev := make(schedStatsMap)
	schedSum := make(schedStatsMap)

	var sysCur *systemStats
	var sysPrev *systemStats
	var sysSum *systemStats
	var sysHist *systemStatsHist

	var t1, t2 time.Time

	// run all scans one time to establish a baseline
	pids := getPidList()

	schedPrev = schedReaderPids(pids)
	t1 = time.Now()
	procPrev = statReader(pids)
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
			pids = getPidList()

			procCur = statReader(pids)
			procDelta := statRecord(procCur, procPrev, procSum, procHist)
			procPrev = procCur

			sysCur = statReaderGlobal()
			sysDelta := statsRecordGlobal(sysCur, sysPrev, sysSum, sysHist)
			sysPrev = sysCur

			if *useTui {
				tuiGraphUpdate(sysDelta, procDelta, topPids)
			}

			t2 = time.Now()
			adjustedSleep = targetSleep - t2.Sub(t1)
		}

		topHist := sortList(procHist, *topN)
		for i := 0; i < *topN; i++ {
			topPids[i] = topHist[i].pid
		}

		schedCur = schedReaderPids(topPids)
		schedRecord(schedCur, schedPrev, schedSum)
		schedPrev = schedCur

		if *useTui {
			tuiListUpdate(topPids, procSum, procHist, sysHist)
		} else {
			dumpStats(topPids, procSum, procHist, sysSum, sysHist, schedSum, jiffy, interval, samples)
		}
		procHist = make(procStatsHistMap)
		procSum = make(procStatsMap)
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
	hist *procStatsHist
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
		float64(m[i].hist.custime.Max()),
		float64(m[i].hist.delayacctBlkioTicks.Max()),
	})
	maxJ := maxList([]float64{
		float64(m[j].hist.ustime.Max()),
		float64(m[j].hist.custime.Max()),
		float64(m[j].hist.delayacctBlkioTicks.Max()),
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

func sortList(histStats procStatsHistMap, limit int) []*sortHist {
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

func formatMem(num uint64) string {
	letter := string("K")

	num = num * 4
	if num >= 10000 {
		num = (num + 512) / 1024
		letter = "M"
		if num >= 10000 {
			num = (num + 512) / 1024
			letter = "G"
		}
	}
	return fmt.Sprintf("%d%s", num, letter)
}

func dumpStats(list pidlist, sumStats procStatsMap, histStats procStatsHistMap, sysSum *systemStats,
	sysHist *systemStatsHist, sumSched schedStatsMap, jiffy, interval, samples *int) {

	scale := func(val float64) float64 {
		return val / float64(*jiffy) / float64(*interval) * 1000 * 100
	}
	scaleSum := func(val float64, count int64) float64 {
		valSec := val / float64(*jiffy)
		sampleSec := float64(*interval) * float64(count) / 1000.0
		ret := (valSec / sampleSec) * 100
		return ret
	}
	scaleSched := func(val float64) float64 {
		return val / float64(*jiffy) / float64((*interval)*(*samples)) * 100
	}

	fmt.Printf("usr:    %4s/%4s/%4s   sys:%4s/%4s/%4s  nice:%4s/%4s/%4s    idle:%4s/%4s/%4s\n",
		trim(scale(float64(sysHist.usr.Min())), 4),
		trim(scale(float64(sysHist.usr.Max())), 4),
		trim(scale(sysHist.usr.Mean()), 4),

		trim(scale(float64(sysHist.sys.Min())), 4),
		trim(scale(float64(sysHist.sys.Max())), 4),
		trim(scale(sysHist.sys.Mean()), 4),

		trim(scale(float64(sysHist.nice.Min())), 4),
		trim(scale(float64(sysHist.nice.Max())), 4),
		trim(scale(sysHist.nice.Mean()), 4),

		trim(scale(float64(sysHist.idle.Min())), 4),
		trim(scale(float64(sysHist.idle.Max())), 4),
		trim(scale(sysHist.idle.Mean()), 4),
	)
	fmt.Printf("iowait: %4s/%4s/%4s  ctxt:%4s/%4s/%4s  prun:%4s/%4s/%4s  pblock:%4s/%4s/%4s  pstart: %4d\n",
		trim(scale(float64(sysHist.iowait.Min())), 4),
		trim(scale(float64(sysHist.iowait.Max())), 4),
		trim(scale(sysHist.iowait.Mean()), 4),

		trim(scale(float64(sysHist.ctxt.Min())), 4),
		trim(scale(float64(sysHist.ctxt.Max())), 4),
		trim(scale(sysHist.ctxt.Mean()), 4),

		trim(float64(sysHist.procsRunning.Min()), 4),
		trim(float64(sysHist.procsRunning.Max()), 4),
		trim(sysHist.procsRunning.Mean(), 4),

		trim(float64(sysHist.procsBlocked.Min()), 4),
		trim(float64(sysHist.procsBlocked.Max()), 4),
		trim(sysHist.procsBlocked.Mean(), 4),

		sysSum.procsTotal,
	)

	fmt.Print("                comm     pid     min     max     usr     sys    nice   ctime    slat     ctx     icx     rss  iowait thrd  sam\n")
	for _, pid := range list {
		hist := histStats[pid]

		var schedWait, nrSwitches, nrInvoluntarySwitches string
		sched, ok := sumSched[pid]
		if ok == true {
			schedWait = trim(scaleSched(sched.waitSum), 7)
			nrSwitches = fmt.Sprintf("%d", sched.nrSwitches)
			nrInvoluntarySwitches = fmt.Sprintf("%d", sched.nrInvoluntarySwitches)
		} else {
			schedWait = "-"
			nrSwitches = "-"
			nrInvoluntarySwitches = "-"
		}
		sampleCount := hist.utime.TotalCount()
		fmt.Printf("%20s %7d %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s %4d %4d\n",
			sumStats[pid].comm,
			pid,
			trim(scale(float64(hist.ustime.Min())), 7),
			trim(scale(float64(hist.ustime.Max())), 7),
			trim(scaleSum(float64(sumStats[pid].utime), sampleCount), 7),
			trim(scaleSum(float64(sumStats[pid].stime), sampleCount), 7),
			trim(float64(sumStats[pid].nice), 7),
			trim(scaleSum(float64(sumStats[pid].cutime+sumStats[pid].cstime), sampleCount), 7),
			schedWait,
			nrSwitches,
			nrInvoluntarySwitches,
			formatMem(sumStats[pid].rss),
			trim(scaleSum(float64(sumStats[pid].delayacctBlkioTicks), sampleCount), 7),
			sumStats[pid].numThreads,
			sampleCount,
		)
	}
	fmt.Println()
}
