package main

import "fmt"

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

func formatNum(num uint64) string {
	if num > 1000000 {
		return fmt.Sprintf("%dM", num/1000000)
	}
	if num > 1000 {
		return fmt.Sprintf("%dK", num/1000)
	}
	return fmt.Sprintf("%d", num)
}

func dumpStats(cmdNames cmdlineMap, list pidlist, sumStats procStatsMap, histStats procStatsHistMap,
	sysSum *systemStats, sysHist *systemStatsHist, sumSched schedStatsMap, jiffy, interval, samples *int) {

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

	fmt.Print("                   comm     pid     min     max     usr     sys   nice   ctime    slat   ctx   icx   rss   iow  thrd  sam\n")
	for _, pid := range list {
		hist := histStats[pid]

		var schedWait, nrSwitches, nrInvoluntarySwitches string
		sched, ok := sumSched[pid]
		if ok == true {
			schedWait = trim(scaleSched(sched.waitSum), 7)
			nrSwitches = formatNum(sched.nrSwitches)
			nrInvoluntarySwitches = formatNum(sched.nrInvoluntarySwitches)
		} else {
			schedWait = "-"
			nrSwitches = "-"
			nrInvoluntarySwitches = "-"
		}
		sampleCount := hist.utime.TotalCount()
		var friendlyName string
		cmdName, ok := cmdNames[pid]
		if ok == true && len(cmdName.friendly) > 0 {
			friendlyName = cmdName.friendly
		} else {
			// This should not happen as long as the cmdline resolver works
			fmt.Println("using comm for ", cmdName, pid)
			friendlyName = sumStats[pid].comm
		}
		if len(friendlyName) > 23 {
			friendlyName = friendlyName[:23]
		}

		fmt.Printf("%23s %7d %7s %7s %7s %7s %6s %7s %7s %5s %5s %5s %5s %5d %4d\n",
			friendlyName,
			pid,
			trim(scale(float64(hist.ustime.Min())), 7),
			trim(scale(float64(hist.ustime.Max())), 7),
			trim(scaleSum(float64(sumStats[pid].utime), sampleCount), 7),
			trim(scaleSum(float64(sumStats[pid].stime), sampleCount), 7),
			trim(float64(sumStats[pid].nice), 6),
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
