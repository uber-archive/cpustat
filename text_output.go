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

package main

import (
	"fmt"
	"strings"
	"time"

	lib "github.com/uber-common/cpustat/lib"
)

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

func textInit(interval, samples, topN int, filters lib.Filters) {
	fmt.Printf("sampling interval:%s, summary interval:%s (%d samples), showing top %d procs,",
		time.Duration(interval)*time.Millisecond,
		time.Duration(interval*samples)*time.Millisecond,
		samples, topN)
	fmt.Print(" user filter:")
	if len(filters.User) == 0 {
		fmt.Print("all")
	} else {
		fmt.Print(strings.Join(filters.UserStr, ","))
	}
	fmt.Print(", pid filter:")
	if len(filters.Pid) == 0 {
		fmt.Print("all")
	} else {
		fmt.Print(strings.Join(filters.PidStr, ","))
	}
	fmt.Println()
}

func dumpStats(infoMap lib.ProcInfoMap, list lib.Pidlist, procSum lib.ProcSampleMap,
	procHist lib.ProcStatsHistMap, taskHist lib.TaskStatsHistMap,
	sysSum *lib.SystemStats, sysHist *lib.SystemStatsHist, jiffy, interval, samples int) {

	scale := func(val float64) float64 {
		return val / float64(jiffy) / float64(interval) * 1000 * 100
	}
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

	fmt.Printf("usr:    %4s/%4s/%4s   sys:%4s/%4s/%4s    nice:%4s/%4s/%4s  idle:%4s/%4s/%4s\n",
		trim(scale(float64(sysHist.Usr.Min())), 4),
		trim(scale(sysHist.Usr.Mean()), 4),
		trim(scale(float64(sysHist.Usr.Max())), 4),

		trim(scale(float64(sysHist.Sys.Min())), 4),
		trim(scale(sysHist.Sys.Mean()), 4),
		trim(scale(float64(sysHist.Sys.Max())), 4),

		trim(scale(float64(sysHist.Nice.Min())), 4),
		trim(scale(sysHist.Nice.Mean()), 4),
		trim(scale(float64(sysHist.Nice.Max())), 4),

		trim(scale(float64(sysHist.Idle.Min())), 4),
		trim(scale(sysHist.Idle.Mean()), 4),
		trim(scale(float64(sysHist.Idle.Max())), 4),
	)
	fmt.Printf("iowait: %4s/%4s/%4s  prun:%4s/%4s/%4s  pblock:%4s/%4s/%4s  pstart: %4d\n",
		trim(scale(float64(sysHist.Iowait.Min())), 4),
		trim(scale(sysHist.Iowait.Mean()), 4),
		trim(scale(float64(sysHist.Iowait.Max())), 4),

		trim(float64(sysHist.ProcsRunning.Min()), 4),
		trim(sysHist.ProcsRunning.Mean(), 4),
		trim(float64(sysHist.ProcsRunning.Max()), 4),

		trim(float64(sysHist.ProcsBlocked.Min()), 4),
		trim(sysHist.ProcsBlocked.Mean(), 4),
		trim(float64(sysHist.ProcsBlocked.Max()), 4),

		sysSum.ProcsTotal,
	)

	fmt.Print("                      name    pid     min     max     usr     sys    runq     iow    swap   vcx   icx   ctime   rss nice thrd  sam\n")
	for _, pid := range list {
		sampleCount := procHist[pid].Ustime.TotalCount()

		var cpuDelay, blockDelay, swapDelay, nvcsw, nivcsw string

		if proc, ok := procSum[pid]; ok == true {
			cpuDelay = trim(scaleSumUs(float64(proc.Task.Cpudelaytotal), sampleCount), 7)
			blockDelay = trim(scaleSumUs(float64(proc.Task.Blkiodelaytotal), sampleCount), 7)
			swapDelay = trim(scaleSumUs(float64(proc.Task.Swapindelaytotal), sampleCount), 7)
			nvcsw = formatNum(proc.Task.Nvcsw)
			nivcsw = formatNum(proc.Task.Nivcsw)
		} else {
			fmt.Println("pid", pid, "missing at sum time")
			continue
		}

		fmt.Printf("%26s %6d %7s %7s %7s %7s %7s %7s %7s %5s %5s %7s %5s %4d %4d %4d\n",
			trunc(infoMap[pid].Friendly, 26),
			pid,
			trim(scale(float64(procHist[pid].Ustime.Min())), 7),
			trim(scale(float64(procHist[pid].Ustime.Max())), 7),
			trim(scaleSum(float64(procSum[pid].Proc.Utime), sampleCount), 7),
			trim(scaleSum(float64(procSum[pid].Proc.Stime), sampleCount), 7),
			cpuDelay,
			blockDelay,
			swapDelay,
			nvcsw,
			nivcsw,
			trim(scaleSum(float64(procSum[pid].Proc.Cutime+procSum[pid].Proc.Cstime), sampleCount), 7),
			formatMem(procSum[pid].Proc.Rss),
			infoMap[pid].Nice,
			procSum[pid].Proc.Numthreads,
			sampleCount,
		)
	}
}
