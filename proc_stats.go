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

// The per-process data from /proc/[pid]/stat

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/codahale/hdrhistogram"
)

type procStats struct {
	captureTime         time.Time
	prevTime            time.Time
	pid                 uint64
	comm                string
	state               string
	ppid                uint64
	pgrp                int64
	session             int64
	ttyNr               int64
	tpgid               int64
	flags               uint64
	minflt              uint64
	cminflt             uint64
	majflt              uint64
	cmajflt             uint64
	utime               uint64
	stime               uint64
	cutime              uint64
	cstime              uint64
	priority            int64
	nice                int64
	numThreads          uint64
	startTime           uint64
	vsize               uint64
	rss                 uint64
	rsslim              uint64
	processor           uint64
	rtPriority          uint64
	policy              uint64
	delayacctBlkioTicks uint64
	guestTime           uint64
	cguestTime          uint64
}

// These are somewhat expensive to track, so only maintain a hist for ones we use
type procStatsHist struct {
	utime   *hdrhistogram.Histogram
	stime   *hdrhistogram.Histogram
	ustime  *hdrhistogram.Histogram // utime + stime
	cutime  *hdrhistogram.Histogram
	cstime  *hdrhistogram.Histogram
	custime *hdrhistogram.Histogram // cutime + cstime
}

type procStatsMap map[int]*procStats
type procStatsHistMap map[int]*procStatsHist

// you might think that we could split on space, but due to what can at best be called
// a shortcoming of the /proc/pid/stat format, the comm field can have spaces, parens, etc.
// and it is unescaped.
// This is a big paranoid, because even many common tools like htop do not handle this case
// well.
func procPidStatSplit(line string) []string {
	line = strings.TrimSpace(line)
	parts := make([]string, 52)

	partnum := 0
	strpos := 0
	start := 0
	inword := false
	space := " "[0]
	open := "("[0]
	close := ")"[0]
	groupchar := space

	for ; strpos < len(line); strpos++ {
		if inword {
			if line[strpos] == space && (groupchar == space || line[strpos-1] == groupchar) {
				parts[partnum] = line[start:strpos]
				partnum++
				start = strpos
				inword = false
			}
		} else {
			if line[strpos] == open {
				groupchar = close
				inword = true
				start = strpos
				strpos = strings.LastIndex(line, ")") - 1
				if strpos <= start { // if we can't parse this insane field, skip to the end
					strpos = len(line)
					inword = false
				}
			} else if line[strpos] != space {
				groupchar = space
				inword = true
				start = strpos
			}
		}
	}

	if inword {
		parts[partnum] = line[start:strpos]
	}

	return parts
}

func procStatsReader(pids pidlist, cmdNames cmdlineMap) procStatsMap {
	cur := make(procStatsMap)

	for _, pid := range pids {
		lines, err := readFileLines(fmt.Sprintf("/proc/%d/stat", pid))
		// pid could have exited between when we scanned the dir and now
		if err != nil {
			continue
		}

		// this format of this file is insane because comm can have split chars in it
		parts := procPidStatSplit(lines[0])

		stat := procStats{
			time.Now(), // this might be expensive. If so, can cache it. We only need 1ms resolution
			time.Time{},
			readUInt(parts[0]),                  // pid
			strings.Map(stripSpecial, parts[1]), // comm
			parts[2],            // state
			readUInt(parts[3]),  // ppid
			readInt(parts[4]),   // pgrp
			readInt(parts[5]),   // session
			readInt(parts[6]),   // tty_nr
			readInt(parts[7]),   // tpgid
			readUInt(parts[8]),  // flags
			readUInt(parts[9]),  // minflt
			readUInt(parts[10]), // cminflt
			readUInt(parts[11]), // majflt
			readUInt(parts[12]), // cmajflt
			readUInt(parts[13]), // utime
			readUInt(parts[14]), // stime
			readUInt(parts[15]), // cutime
			readUInt(parts[16]), // cstime
			readInt(parts[17]),  // priority
			readInt(parts[18]),  // nice
			readUInt(parts[19]), // num_threads
			// itrealvalue - not maintained
			readUInt(parts[21]), // starttime
			readUInt(parts[22]), // vsize
			readUInt(parts[23]), // rss
			readUInt(parts[24]), // rsslim
			// bunch of stuff about memory addresses
			readUInt(parts[38]), // processor
			readUInt(parts[39]), // rt_priority
			readUInt(parts[40]), // policy
			readUInt(parts[41]), // delayacct_blkio_ticks
			readUInt(parts[42]), // guest_time
			readUInt(parts[43]), // cguest_time
		}
		cur[pid] = &stat
		updateCmdline(cmdNames, pid, stat.comm)
	}
	return cur
}

func procStatsRecord(interval int, curMap, prevMap, sumMap procStatsMap, histMap procStatsHistMap) procStatsMap {
	deltaMap := make(procStatsMap)

	for pid, cur := range curMap {
		if prev, ok := prevMap[pid]; ok == true {
			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &procStats{}
			}
			deltaMap[pid] = &procStats{}
			delta := deltaMap[pid]

			delta.captureTime = cur.captureTime
			delta.prevTime = prev.captureTime
			duration := float64(delta.captureTime.Sub(delta.prevTime) / time.Millisecond)
			scale := float64(interval) / duration

			sum := sumMap[pid]
			sum.captureTime = cur.captureTime
			sum.pid = cur.pid
			sum.comm = cur.comm
			sum.state = cur.state
			sum.ppid = cur.ppid
			sum.pgrp = cur.pgrp
			sum.session = cur.session
			sum.ttyNr = cur.ttyNr
			sum.tpgid = cur.tpgid
			sum.flags = cur.flags
			delta.minflt = scaledSub(cur.minflt, prev.minflt, scale)
			sum.minflt += safeSub(cur.minflt, prev.minflt)
			delta.cminflt = scaledSub(cur.cminflt, prev.cminflt, scale)
			sum.cminflt += safeSub(cur.cminflt, prev.cminflt)
			delta.majflt = scaledSub(cur.majflt, prev.majflt, scale)
			sum.majflt += safeSub(cur.majflt, prev.majflt)
			delta.cmajflt = scaledSub(cur.cmajflt, prev.cmajflt, scale)
			sum.cmajflt += safeSub(cur.cmajflt, prev.cmajflt)
			delta.utime = scaledSub(cur.utime, prev.utime, scale)
			sum.utime += safeSub(cur.utime, prev.utime)
			delta.stime = scaledSub(cur.stime, prev.stime, scale)
			sum.stime += safeSub(cur.stime, prev.stime)
			delta.cutime = scaledSub(cur.cutime, prev.cutime, scale)
			sum.cutime += safeSub(cur.cutime, prev.cutime)
			delta.cstime = scaledSub(cur.cstime, prev.cstime, scale)
			sum.cstime += safeSub(cur.cstime, prev.cstime)
			sum.priority = cur.priority
			sum.nice = cur.nice
			sum.numThreads = cur.numThreads
			sum.startTime = cur.startTime
			sum.vsize = cur.vsize
			sum.rss = cur.rss
			sum.rsslim = cur.rsslim
			sum.processor = cur.processor
			sum.rtPriority = cur.rtPriority
			sum.policy = cur.policy
			delta.delayacctBlkioTicks = scaledSub(cur.delayacctBlkioTicks, prev.delayacctBlkioTicks, scale)
			sum.delayacctBlkioTicks += safeSub(cur.delayacctBlkioTicks, prev.delayacctBlkioTicks)
			delta.guestTime = scaledSub(cur.guestTime, prev.guestTime, scale)
			sum.guestTime += safeSub(cur.guestTime, prev.guestTime)

			var hist *procStatsHist
			if hist, ok = histMap[pid]; ok != true {
				histMap[pid] = &procStatsHist{
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
				}
				hist = histMap[pid]
			}
			hist.utime.RecordValue(int64(delta.utime))
			hist.stime.RecordValue(int64(delta.stime))
			hist.ustime.RecordValue(int64(delta.utime + delta.stime))
			hist.cutime.RecordValue(int64(delta.cutime))
			hist.cstime.RecordValue(int64(delta.cstime))
			hist.custime.RecordValue(int64(delta.cutime + delta.cstime))
		}
	}

	return deltaMap
}
