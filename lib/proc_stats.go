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

// per-process data from /proc/[pid]/stat

package cpustat

import (
	"fmt"
	"strings"
	"time"
)

// ProcStats holds the information that comes back from /proc/[pid]/stat
type ProcStats struct {
	CaptureTime         time.Time
	Pid                 uint64
	Comm                string
	State               string
	Ppid                uint64
	Pgrp                int64
	Session             int64
	Ttynr               int64
	Tpgid               int64
	Flags               uint64
	Minflt              uint64
	Cminflt             uint64
	Majflt              uint64
	Cmajflt             uint64
	Utime               uint64
	Stime               uint64
	Cutime              uint64
	Cstime              uint64
	Priority            int64
	Nice                int64
	Numthreads          uint64
	Starttime           uint64
	Vsize               uint64
	Rss                 uint64
	Rsslim              uint64
	Processor           uint64
	Rtpriority          uint64
	Policy              uint64
	Delayacctblkioticks uint64
	Guesttime           uint64
	Cguesttime          uint64
}

type ProcStatsMap map[int]*ProcStats

// super not thread safe but GC friendly way to reuse this string slice
var splitParts []string

// you might think that we could split on space, but due to what can at best be called
// a shortcoming of the /proc/pid/stat format, the comm field can have unescaped spaces, parens, etc.
// This may be a bit paranoid, because even many common tools like htop do not handle this case well.
func procPidStatSplit(line string) []string {
	line = strings.TrimSpace(line)

	if splitParts == nil {
		splitParts = make([]string, 52)
	}

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
				splitParts[partnum] = line[start:strpos]
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
		splitParts[partnum] = line[start:strpos]
		partnum++
	}

	for ; partnum < 52; partnum++ {
		splitParts[partnum] = ""
	}
	return splitParts
}

// ProcStatsReader reads and parses /proc/[pid]/stat for all of pids
func ProcStatsReader(pids Pidlist, cmdNames CmdlineMap) ProcStatsMap {
	cur := make(ProcStatsMap)

	for _, pid := range pids {
		lines, err := ReadFileLines(fmt.Sprintf("/proc/%d/stat", pid))
		// pid could have exited between when we scanned the dir and now
		if err != nil {
			continue
		}

		// this format of this file is insane because comm can have split chars in it
		parts := procPidStatSplit(lines[0])

		stat := ProcStats{
			time.Now(),
			ReadUInt(parts[0]),                  // pid
			strings.Map(StripSpecial, parts[1]), // comm
			parts[2],            // state
			ReadUInt(parts[3]),  // ppid
			ReadInt(parts[4]),   // pgrp
			ReadInt(parts[5]),   // session
			ReadInt(parts[6]),   // tty_nr
			ReadInt(parts[7]),   // tpgid
			ReadUInt(parts[8]),  // flags
			ReadUInt(parts[9]),  // minflt
			ReadUInt(parts[10]), // cminflt
			ReadUInt(parts[11]), // majflt
			ReadUInt(parts[12]), // cmajflt
			ReadUInt(parts[13]), // utime
			ReadUInt(parts[14]), // stime
			ReadUInt(parts[15]), // cutime
			ReadUInt(parts[16]), // cstime
			ReadInt(parts[17]),  // priority
			ReadInt(parts[18]),  // nice
			ReadUInt(parts[19]), // num_threads
			// itrealvalue - not maintained
			ReadUInt(parts[21]), // starttime
			ReadUInt(parts[22]), // vsize
			ReadUInt(parts[23]), // rss
			ReadUInt(parts[24]), // rsslim
			// bunch of stuff about memory addresses
			ReadUInt(parts[38]), // processor
			ReadUInt(parts[39]), // rt_priority
			ReadUInt(parts[40]), // policy
			ReadUInt(parts[41]), // delayacct_blkio_ticks
			ReadUInt(parts[42]), // guest_time
			ReadUInt(parts[43]), // cguest_time
		}
		cur[pid] = &stat
		updateCmdline(cmdNames, pid, stat.Comm)
	}
	return cur
}

// compute the delta between this sample and the previous one.
func ProcStatsRecord(interval int, curMap, prevMap, sumMap ProcStatsMap) ProcStatsMap {
	deltaMap := make(ProcStatsMap)

	for pid, cur := range curMap {
		if prev, ok := prevMap[pid]; ok == true {
			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &ProcStats{}
			}
			deltaMap[pid] = &ProcStats{}
			delta := deltaMap[pid]

			delta.CaptureTime = cur.CaptureTime
			duration := float64(delta.CaptureTime.Sub(prev.CaptureTime) / time.Millisecond)
			scale := float64(interval) / duration

			sum := sumMap[pid]
			sum.CaptureTime = cur.CaptureTime
			sum.Pid = cur.Pid
			sum.Comm = cur.Comm
			sum.State = cur.State
			sum.Ppid = cur.Ppid
			sum.Pgrp = cur.Pgrp
			sum.Session = cur.Session
			sum.Ttynr = cur.Ttynr
			sum.Tpgid = cur.Tpgid
			sum.Flags = cur.Flags
			delta.Minflt = ScaledSub(cur.Minflt, prev.Minflt, scale)
			sum.Minflt += SafeSub(cur.Minflt, prev.Minflt)
			delta.Cminflt = ScaledSub(cur.Cminflt, prev.Cminflt, scale)
			sum.Cminflt += SafeSub(cur.Cminflt, prev.Cminflt)
			delta.Majflt = ScaledSub(cur.Majflt, prev.Majflt, scale)
			sum.Majflt += SafeSub(cur.Majflt, prev.Majflt)
			delta.Cmajflt = ScaledSub(cur.Cmajflt, prev.Cmajflt, scale)
			sum.Cmajflt += SafeSub(cur.Cmajflt, prev.Cmajflt)
			delta.Utime = ScaledSub(cur.Utime, prev.Utime, scale)
			sum.Utime += SafeSub(cur.Utime, prev.Utime)
			delta.Stime = ScaledSub(cur.Stime, prev.Stime, scale)
			sum.Stime += SafeSub(cur.Stime, prev.Stime)
			delta.Cutime = ScaledSub(cur.Cutime, prev.Cutime, scale)
			sum.Cutime += SafeSub(cur.Cutime, prev.Cutime)
			delta.Cstime = ScaledSub(cur.Cstime, prev.Cstime, scale)
			sum.Cstime += SafeSub(cur.Cstime, prev.Cstime)
			sum.Priority = cur.Priority
			sum.Nice = cur.Nice
			sum.Numthreads = cur.Numthreads
			sum.Starttime = cur.Starttime
			sum.Vsize = cur.Vsize
			sum.Rss = cur.Rss
			sum.Rsslim = cur.Rsslim
			sum.Processor = cur.Processor
			sum.Rtpriority = cur.Rtpriority
			sum.Policy = cur.Policy
			delta.Delayacctblkioticks = ScaledSub(cur.Delayacctblkioticks, prev.Delayacctblkioticks, scale)
			sum.Delayacctblkioticks += SafeSub(cur.Delayacctblkioticks, prev.Delayacctblkioticks)
			delta.Guesttime = ScaledSub(cur.Guesttime, prev.Guesttime, scale)
			sum.Guesttime += SafeSub(cur.Guesttime, prev.Guesttime)
		}
	}

	return deltaMap
}
