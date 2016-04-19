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

import "time"

// TaskStats is the set of data from linux/taskstats.h that seemed relevant and accurate.
// There are other things in the kernel struct that are not tracked here, but perhaps should be.
type TaskStats struct {
	Capturetime         time.Time
	Cpudelaycount       uint64 // delay count waiting for CPU, while runnable
	Cpudelaytotal       uint64 // delay time waiting for CPU, while runnable, in ns
	Blkiodelaycount     uint64 // delay count waiting for disk
	Blkiodelaytotal     uint64 // delay time waiting for disk
	Swapindelaycount    uint64 // delay count waiting for swap
	Swapindelaytotal    uint64 // delay time waiting for swap
	Nvcsw               uint64 // voluntary context switches
	Nivcsw              uint64 // involuntary context switches
	Freepagesdelaycount uint64 // delay count waiting for memory reclaim
	Freepagesdelaytotal uint64 // delay time waiting for memory reclaim in unknown units
}

// TaskStatsMap maps pid to TaskStats, suually representing a sample of all pids
type TaskStatsMap map[int]*TaskStats

// TaskStatsReader uses conn to build a TaskStatsMap for all pids.
func TaskStatsReader(conn *NLConn, pids Pidlist, cur *ProcSampleList) {
	for i := uint32(0); i < cur.Len; i++ {
		err := TaskStatsLookupPid(conn, &cur.Samples[i])
		// it would be better to check for other errors here
		if err != nil {
			continue
		}
	}
}

// TaskStatsRecord computes the delta between Task elements of two ProcSampleLists
// These lists do not need to have exactly the same processes in it, but they must both be sorted by Pid.
// This generally works out because reading the pids from /proc puts them in a consistent order.
// If we ever get a new source of the pidlist, perf_events or whatever, make sure it sorts.
func TaskStatsRecord(interval uint32, curList, prevList ProcSampleList, sumMap, deltaMap ProcSampleMap) {
	curPos := uint32(0)
	prevPos := uint32(0)

	for curPos < curList.Len && prevPos < prevList.Len {
		if curList.Samples[curPos].Pid == prevList.Samples[prevPos].Pid {
			cur := &(curList.Samples[curPos].Task)
			prev := &(prevList.Samples[prevPos].Task)
			pid := curList.Samples[curPos].Pid

			delta := &(deltaMap[pid].Task)

			delta.Capturetime = cur.Capturetime
			duration := float64(cur.Capturetime.Sub(prev.Capturetime) / time.Millisecond)
			scale := float64(interval) / duration

			// sumMap[pid] needs to exist
			sum := &(sumMap[pid].Task)
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
			delta.Nvcsw = ScaledSub(cur.Nvcsw, prev.Nvcsw, scale)
			sum.Nvcsw += SafeSub(cur.Nvcsw, prev.Nvcsw)
			delta.Nivcsw = ScaledSub(cur.Nivcsw, prev.Nivcsw, scale)
			sum.Nivcsw += SafeSub(cur.Nivcsw, prev.Nivcsw)
			delta.Freepagesdelaycount = ScaledSub(cur.Freepagesdelaycount, prev.Freepagesdelaycount, scale)
			sum.Freepagesdelaycount += SafeSub(cur.Freepagesdelaycount, prev.Freepagesdelaycount)
			delta.Freepagesdelaytotal = ScaledSub(cur.Freepagesdelaytotal, prev.Freepagesdelaytotal, scale)
			sum.Freepagesdelaytotal += SafeSub(cur.Freepagesdelaytotal, prev.Freepagesdelaytotal)
			curPos++
			prevPos++
		} else {
			if curList.Samples[curPos].Pid < prevList.Samples[prevPos].Pid {
				curPos++
			} else {
				prevPos++
			}
		}
	}
}
