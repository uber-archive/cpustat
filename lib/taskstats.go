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

package cpustat

import (
	"time"

	"github.com/codahale/hdrhistogram"
)

type TaskStats struct {
	Capturetime         time.Time
	Cpudelaycount       uint64 // delay count waiting for CPU, while runnable
	Cpudelaytotal       uint64 // delay time waiting for CPU, while runnable, in ns
	Blkiodelaycount     uint64 // delay count waiting for disk
	Blkiodelaytotal     uint64 // delay time waiting for disk
	Swapindelaycount    uint64 // delay count waiting for swap
	Swapindelaytotal    uint64 // delay time waiting for swap
	Minflt              uint64 // major page fault count
	Majflt              uint64 // minor page fault count
	Readchar            uint64 // total bytes read
	Writechar           uint64 // total bytes written
	Readsyscalls        uint64 // read system calls
	Writesyscalls       uint64 // write system calls
	Readbytes           uint64 // bytes read total
	Writebytes          uint64 // bytes written total
	Cancelledwritebytes uint64 // bytes of cancelled write IO, whatever that is
	Nvcsw               uint64 // voluntary context switches
	Nivcsw              uint64 // involuntary context switches
	Freepagesdelaycount uint64 // delay count waiting for memory reclaim
	Freepagesdelaytotal uint64 // delay time waiting for memory reclaim in unknown units
}

type TaskStatsHist struct {
	cpudelay *hdrhistogram.Histogram
	iowait   *hdrhistogram.Histogram
	swap     *hdrhistogram.Histogram
}

type TaskStatsMap map[int]*TaskStats
type TaskStatsHistMap map[int]*TaskStatsHist

func TaskStatsReader(conn *NLConn, pids Pidlist, cmdNames CmdlineMap) TaskStatsMap {
	cur := make(TaskStatsMap)

	for _, pid := range pids {
		stat, comm, err := TaskStatsLookupPid(conn, pid)
		// it would be better to check for other errors here
		if err != nil {
			continue
		}
		cur[pid] = stat
		updateCmdline(cmdNames, pid, comm)
	}
	return cur
}

func TaskStatsRecord(interval int, curMap, prevMap, sumMap TaskStatsMap, histMap TaskStatsHistMap) TaskStatsMap {
	deltaMap := make(TaskStatsMap)

	for pid, cur := range curMap {
		if prev, ok := prevMap[pid]; ok == true {
			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &TaskStats{}
			}
			deltaMap[pid] = &TaskStats{}
			delta := deltaMap[pid]

			delta.Capturetime = cur.Capturetime
			duration := float64(cur.Capturetime.Sub(prev.Capturetime) / time.Millisecond)
			scale := float64(interval) / duration

			sum := sumMap[pid]
			sum.Capturetime = cur.Capturetime

			delta.Cpudelaycount = ScaledSub(cur.Cpudelaycount, prev.Cpudelaycount, scale)
			sum.Cpudelaycount += SafeSub(cur.Cpudelaycount, prev.Cpudelaycount)
			delta.Cpudelaytotal = ScaledSub(cur.Cpudelaytotal, prev.Cpudelaytotal, scale)
			sum.Cpudelaytotal += SafeSub(cur.Cpudelaytotal, prev.Cpudelaytotal)
			delta.Blkiodelaycount = ScaledSub(cur.Blkiodelaycount, prev.Blkiodelaycount, scale)
			sum.Blkiodelaycount += SafeSub(cur.Blkiodelaycount, prev.Blkiodelaycount)
			delta.Blkiodelaytotal = ScaledSub(cur.Blkiodelaytotal, prev.Blkiodelaytotal, scale)
			sum.Blkiodelaytotal += SafeSub(cur.Blkiodelaytotal, prev.Blkiodelaytotal)
			delta.Swapindelaycount = ScaledSub(cur.Swapindelaycount, prev.Swapindelaycount, scale)
			sum.Swapindelaycount += SafeSub(cur.Swapindelaycount, prev.Swapindelaycount)
			delta.Swapindelaytotal = ScaledSub(cur.Swapindelaytotal, prev.Swapindelaytotal, scale)
			sum.Swapindelaytotal += SafeSub(cur.Swapindelaytotal, prev.Swapindelaytotal)
			delta.Minflt = ScaledSub(cur.Minflt, prev.Minflt, scale)
			sum.Minflt += SafeSub(cur.Minflt, prev.Minflt)
			delta.Majflt = ScaledSub(cur.Majflt, prev.Majflt, scale)
			sum.Majflt += SafeSub(cur.Majflt, prev.Majflt)
			delta.Readchar = ScaledSub(cur.Readchar, prev.Readchar, scale)
			sum.Readchar += SafeSub(cur.Readchar, prev.Readchar)
			delta.Writechar = ScaledSub(cur.Writechar, prev.Writechar, scale)
			sum.Writechar += SafeSub(cur.Writechar, prev.Writechar)
			delta.Readsyscalls = ScaledSub(cur.Readsyscalls, prev.Readsyscalls, scale)
			sum.Readsyscalls += SafeSub(cur.Readsyscalls, prev.Readsyscalls)
			delta.Writesyscalls = ScaledSub(cur.Writesyscalls, prev.Writesyscalls, scale)
			sum.Writesyscalls += SafeSub(cur.Writesyscalls, prev.Writesyscalls)
			delta.Readbytes = ScaledSub(cur.Readbytes, prev.Readbytes, scale)
			sum.Readbytes += SafeSub(cur.Readbytes, prev.Readbytes)
			delta.Writebytes = ScaledSub(cur.Writebytes, prev.Writebytes, scale)
			sum.Writebytes += SafeSub(cur.Writebytes, prev.Writebytes)
			delta.Cancelledwritebytes = ScaledSub(cur.Cancelledwritebytes, prev.Cancelledwritebytes, scale)
			sum.Cancelledwritebytes += SafeSub(cur.Cancelledwritebytes, prev.Cancelledwritebytes)
			delta.Nvcsw = ScaledSub(cur.Nvcsw, prev.Nvcsw, scale)
			sum.Nvcsw += SafeSub(cur.Nvcsw, prev.Nvcsw)
			delta.Nivcsw = ScaledSub(cur.Nivcsw, prev.Nivcsw, scale)
			sum.Nivcsw += SafeSub(cur.Nivcsw, prev.Nivcsw)
			delta.Freepagesdelaycount = ScaledSub(cur.Freepagesdelaycount, prev.Freepagesdelaycount, scale)
			sum.Freepagesdelaycount += SafeSub(cur.Freepagesdelaycount, prev.Freepagesdelaycount)
			delta.Freepagesdelaytotal = ScaledSub(cur.Freepagesdelaytotal, prev.Freepagesdelaytotal, scale)
			sum.Freepagesdelaytotal += SafeSub(cur.Freepagesdelaytotal, prev.Freepagesdelaytotal)
		}
	}

	return deltaMap
}
