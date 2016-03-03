package main

import "fmt"

func formatMem(num uint64) string {
	letter := string("K")

	num = num * 4
	if num >= 1000 {
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

func trim(num float64, max int) string {
	var str string
	if num >= 1000.0 {
		str = fmt.Sprintf("%d", int(num+0.5))
	} else {
		str = fmt.Sprintf("%.1f", num)
	}
	if len(str) > max {
		if str[max-1] == 46 { // ASCII .
			return str[:max-1]
		}
		return str[:max]
	}
	if str == "0.0" {
		return "0"
	}
	return str
}

func trunc(str string, length int) string {
	if len(str) <= length {
		return str
	}
	return str[:length]
}

func dumpStats(cmdNames cmdlineMap, list pidlist, procSum procStatsMap, procHist procStatsHistMap,
	taskSum taskStatsMap, taskHist taskStatsHistMap, sysSum *systemStats, sysHist *systemStatsHist,
	jiffy, interval, samples int) {

	scale := func(val float64) float64 {
		return val / float64(jiffy) / float64(interval) * 1000 * 100
	}
	// scaleUs := func(val float64) float64 {
	// 	return val / 1000 / float64(interval) * 100
	// }
	scaleSum := func(val float64, count int64) float64 {
		valSec := val / float64(jiffy)
		sampleSec := float64(interval) * float64(count) / 1000.0
		ret := (valSec / sampleSec) * 100
		return ret
	}
	scaleSumUs := func(val float64, count int64) float64 {
		valSec := val / 1000 / 1000 / float64(interval)
		sampleSec := float64(interval) * float64(count) / 1000.0
		return (valSec / sampleSec) * 100
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

	fmt.Print("                   comm     pid     min     max     usr     sys  nice    runq     iow    swap   ctx   icx   rss   ctime thrd  sam\n")
	for _, pid := range list {
		sampleCount := procHist[pid].ustime.TotalCount()

		var cpuDelay, blockDelay, swapDelay, nvcsw, nivcsw string

		if task, ok := taskSum[pid]; ok == true {
			cpuDelay = trim(scaleSumUs(float64(task.cpudelaytotal), sampleCount), 7)
			blockDelay = trim(scaleSumUs(float64(task.blkiodelaytotal), sampleCount), 7)
			swapDelay = trim(scaleSumUs(float64(task.swapindelaytotal), sampleCount), 7)
			nvcsw = formatNum(task.nvcsw)
			nivcsw = formatNum(task.nivcsw)
		}

		fmt.Printf("%23s %7d %7s %7s %7s %7s %5d %7s %7s %7s %5s %5s %5s %7s %4d %4d\n",
			trunc(cmdNames[pid].friendly, 23),
			pid,
			trim(scale(float64(procHist[pid].ustime.Min())), 7),
			trim(scale(float64(procHist[pid].ustime.Max())), 7),
			trim(scaleSum(float64(procSum[pid].utime), sampleCount), 7),
			trim(scaleSum(float64(procSum[pid].stime), sampleCount), 7),
			procSum[pid].nice,
			cpuDelay,
			blockDelay,
			swapDelay,
			nvcsw,
			nivcsw,
			formatMem(procSum[pid].rss),
			trim(scaleSum(float64(procSum[pid].cutime+procSum[pid].cstime), sampleCount), 7),
			procSum[pid].numThreads,
			sampleCount,
		)
	}
}
