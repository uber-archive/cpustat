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
	"time"

	"github.com/codahale/hdrhistogram"
)

type taskStatsHist struct {
	cpudelay *hdrhistogram.Histogram
	iowait   *hdrhistogram.Histogram
	swap     *hdrhistogram.Histogram
}

type taskStatsMap map[int]*taskStats
type taskStatsHistMap map[int]*taskStatsHist

func taskStatsReader(conn *NLConn, pids pidlist, cmdNames cmdlineMap) taskStatsMap {
	cur := make(taskStatsMap)

	for _, pid := range pids {
		stat, err := taskstatsLookupPid(conn, pid)
		// it would be better to check for other errors here
		if err != nil {
			continue
		}
		cur[pid] = stat
		updateCmdline(cmdNames, pid, stat.comm)
	}
	return cur
}

func taskStatsRecord(interval int, curMap, prevMap, sumMap taskStatsMap, histMap taskStatsHistMap) taskStatsMap {
	deltaMap := make(taskStatsMap)

	for pid, cur := range curMap {
		if prev, ok := prevMap[pid]; ok == true {
			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &taskStats{}
			}
			deltaMap[pid] = &taskStats{}
			delta := deltaMap[pid]

			delta.captureTime = cur.captureTime
			delta.prevTime = prev.captureTime
			duration := float64(delta.captureTime.Sub(delta.prevTime) / time.Millisecond)
			scale := float64(interval) / duration

			sum := sumMap[pid]
			sum.captureTime = cur.captureTime

			sum.version = cur.version
			sum.exitcode = cur.exitcode
			sum.flag = cur.flag
			sum.nice = cur.nice
			delta.cpudelaycount = scaledSub(cur.cpudelaycount, prev.cpudelaycount, scale)
			sum.cpudelaycount += safeSub(cur.cpudelaycount, prev.cpudelaycount)
			delta.cpudelaytotal = scaledSub(cur.cpudelaytotal, prev.cpudelaytotal, scale)
			sum.cpudelaytotal += safeSub(cur.cpudelaytotal, prev.cpudelaytotal)
			delta.blkiodelaycount = scaledSub(cur.blkiodelaycount, prev.blkiodelaycount, scale)
			sum.blkiodelaycount += safeSub(cur.blkiodelaycount, prev.blkiodelaycount)
			delta.blkiodelaytotal = scaledSub(cur.blkiodelaytotal, prev.blkiodelaytotal, scale)
			sum.blkiodelaytotal += safeSub(cur.blkiodelaytotal, prev.blkiodelaytotal)
			delta.swapindelaycount = scaledSub(cur.swapindelaycount, prev.swapindelaycount, scale)
			sum.swapindelaycount += safeSub(cur.swapindelaycount, prev.swapindelaycount)
			delta.swapindelaytotal = scaledSub(cur.swapindelaytotal, prev.swapindelaytotal, scale)
			sum.swapindelaytotal += safeSub(cur.swapindelaytotal, prev.swapindelaytotal)
			delta.cpurunrealtotal = scaledSub(cur.cpurunrealtotal, prev.cpurunrealtotal, scale)
			sum.cpurunrealtotal += safeSub(cur.cpurunrealtotal, prev.cpurunrealtotal)
			delta.cpurunvirtualtotal = scaledSub(cur.cpurunvirtualtotal, prev.cpurunvirtualtotal, scale)
			sum.cpurunvirtualtotal += safeSub(cur.cpurunvirtualtotal, prev.cpurunvirtualtotal)
			sum.comm = cur.comm
			sum.sched = cur.sched
			sum.uid = cur.uid
			sum.gid = cur.gid
			sum.pid = cur.pid
			sum.ppid = cur.ppid
			sum.btime = cur.btime
			delta.etime = scaledSub(cur.etime, prev.etime, scale)
			sum.etime += safeSub(cur.etime, prev.etime)
			delta.utime = scaledSub(cur.utime, prev.utime, scale)
			sum.utime += safeSub(cur.utime, prev.utime)
			delta.stime = scaledSub(cur.stime, prev.stime, scale)
			sum.stime += safeSub(cur.stime, prev.stime)
			delta.minflt = scaledSub(cur.minflt, prev.minflt, scale)
			sum.minflt += safeSub(cur.minflt, prev.minflt)
			delta.majflt = scaledSub(cur.majflt, prev.majflt, scale)
			sum.majflt += safeSub(cur.majflt, prev.majflt)
			delta.coremem = scaledSub(cur.coremem, prev.coremem, scale)
			sum.coremem += safeSub(cur.coremem, prev.coremem)
			delta.virtmem = scaledSub(cur.virtmem, prev.virtmem, scale)
			sum.virtmem += safeSub(cur.virtmem, prev.virtmem)
			sum.hiwaterrss = cur.hiwaterrss
			sum.hiwatervm = cur.hiwatervm
			delta.readchar = scaledSub(cur.readchar, prev.readchar, scale)
			sum.readchar += safeSub(cur.readchar, prev.readchar)
			delta.writechar = scaledSub(cur.writechar, prev.writechar, scale)
			sum.writechar += safeSub(cur.writechar, prev.writechar)
			delta.readsyscalls = scaledSub(cur.readsyscalls, prev.readsyscalls, scale)
			sum.readsyscalls += safeSub(cur.readsyscalls, prev.readsyscalls)
			delta.writesyscalls = scaledSub(cur.writesyscalls, prev.writesyscalls, scale)
			sum.writesyscalls += safeSub(cur.writesyscalls, prev.writesyscalls)
			delta.readbytes = scaledSub(cur.readbytes, prev.readbytes, scale)
			sum.readbytes += safeSub(cur.readbytes, prev.readbytes)
			delta.writebytes = scaledSub(cur.writebytes, prev.writebytes, scale)
			sum.writebytes += safeSub(cur.writebytes, prev.writebytes)
			delta.cancelledwritebytes = scaledSub(cur.cancelledwritebytes, prev.cancelledwritebytes, scale)
			sum.cancelledwritebytes += safeSub(cur.cancelledwritebytes, prev.cancelledwritebytes)
			delta.nvcsw = scaledSub(cur.nvcsw, prev.nvcsw, scale)
			sum.nvcsw += safeSub(cur.nvcsw, prev.nvcsw)
			delta.nivcsw = scaledSub(cur.nivcsw, prev.nivcsw, scale)
			sum.nivcsw += safeSub(cur.nivcsw, prev.nivcsw)
			delta.utimescaled = scaledSub(cur.utimescaled, prev.utimescaled, scale)
			sum.utimescaled += safeSub(cur.utimescaled, prev.utimescaled)
			delta.stimescaled = scaledSub(cur.stimescaled, prev.stimescaled, scale)
			sum.stimescaled += safeSub(cur.stimescaled, prev.stimescaled)
			delta.cpuscaledrunrealtotal = scaledSub(cur.cpuscaledrunrealtotal, prev.cpuscaledrunrealtotal, scale)
			sum.cpuscaledrunrealtotal += safeSub(cur.cpuscaledrunrealtotal, prev.cpuscaledrunrealtotal)
			delta.freepagescount = scaledSub(cur.freepagescount, prev.freepagescount, scale)
			sum.freepagescount += safeSub(cur.freepagescount, prev.freepagescount)
			delta.freepagesdelaytotal = scaledSub(cur.freepagesdelaytotal, prev.freepagesdelaytotal, scale)
			sum.freepagesdelaytotal += safeSub(cur.freepagesdelaytotal, prev.freepagesdelaytotal)

			var hist *taskStatsHist
			if hist, ok = histMap[pid]; ok != true {
				histMap[pid] = &taskStatsHist{
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
				}
				hist = histMap[pid]
			}
			hist.cpudelay.RecordValue(int64(delta.cpudelaytotal))
			hist.iowait.RecordValue(int64(delta.blkiodelaytotal))
			hist.swap.RecordValue(int64(delta.swapindelaytotal))
		}
	}

	return deltaMap
}
