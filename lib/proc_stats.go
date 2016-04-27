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

type ProcSample struct {
	Pid  int
	Proc ProcStats
	Task TaskStats
}

type ProcSampleList struct {
	Samples []ProcSample
	Len     uint32
}

func NewProcSampleList(size int) ProcSampleList {
	return ProcSampleList{
		make([]ProcSample, size),
		0,
	}
}

type ProcSampleMap map[int]*ProcSample

// ProcStats holds the fast changing data that comes back from /proc/[pid]/stat
// These fields are documented in the linux proc(5) man page
// There are many more of these fields that don't change very often. These are stored in the Cmdline struct.
type ProcStats struct {
	CaptureTime time.Time
	Utime       uint64
	Stime       uint64
	Cutime      uint64
	Cstime      uint64
	Numthreads  uint64
	Rss         uint64
	Guesttime   uint64
	Cguesttime  uint64
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
func ProcStatsReader(pids Pidlist, filter Filters, cur *ProcSampleList, infoMap ProcInfoMap) {
	sampleNum := 0
	pidNum := 0
	for pidNum < len(pids) {
		pid := pids[pidNum]
		pidNum++

		if filter.PidMatch(pid) == false {
			continue
		}
		newPid := false

		// we don't know the userid of this proc to filter until we read/stat /proc/pid/cmdline
		// Do this only when we find a pid for the first time so we don't have to stat as much
		var info *ProcInfo
		var ok bool
		if info, ok = infoMap[pid]; ok == true {
			info.touch()
		} else {
			newPid = true
			info = &ProcInfo{}
			info.init()
		}

		lines, err := ReadFileLines(fmt.Sprintf("/proc/%d/stat", pid))
		// pid could have exited between when we scanned the dir and now
		if err != nil {
			continue
		}

		// this format of this file is insane because comm can have split chars in it
		parts := procPidStatSplit(lines[0])

		if newPid {
			info.Comm = strings.Map(StripSpecial, parts[1])
			info.Pid = uint64(pid)
			info.Ppid = ReadUInt(parts[3])
			info.Pgrp = ReadInt(parts[4])
			info.Session = ReadInt(parts[5])
			info.Ttynr = ReadInt(parts[6])
			info.Tpgid = ReadInt(parts[7])
			info.Flags = ReadUInt(parts[8])
			info.Starttime = ReadUInt(parts[21])
			info.Nice = ReadInt(parts[18])
			info.Rtpriority = ReadUInt(parts[39])
			info.Policy = ReadUInt(parts[40])
			info.updateCmdline() // note that this may leave UID at 0 if there's an error
			infoMap[pid] = info
		}

		if filter.UserMatch(int(info.UID)) == false {
			continue
		}

		sample := &cur.Samples[sampleNum]
		sample.Pid = pid
		sample.Proc.CaptureTime = time.Now()
		sample.Proc.Utime = ReadUInt(parts[13])
		sample.Proc.Stime = ReadUInt(parts[14])
		sample.Proc.Cutime = ReadUInt(parts[15])
		sample.Proc.Cstime = ReadUInt(parts[16])
		sample.Proc.Numthreads = ReadUInt(parts[19])
		sample.Proc.Rss = ReadUInt(parts[23])
		sample.Proc.Guesttime = ReadUInt(parts[42])
		sample.Proc.Cguesttime = ReadUInt(parts[43])
		sampleNum++
	}
	cur.Len = uint32(sampleNum)
}

// ProcStatsRecord computes the delta between the Proc elements of two ProcSampleLists
// These lists do not need to have exactly the same processes in it, but they must both be sorted by Pid.
// This generally works out because reading the pids from /proc puts them in a consistent order.
// If we ever get a new source of the pidlist, perf_events or whatever, make sure it sorts.
func ProcStatsRecord(interval uint32, curList, prevList ProcSampleList, sumMap, deltaMap ProcSampleMap) {

	curPos := uint32(0)
	prevPos := uint32(0)

	for curPos < curList.Len && prevPos < prevList.Len {
		if curList.Samples[curPos].Pid == prevList.Samples[prevPos].Pid {
			cur := &(curList.Samples[curPos].Proc)
			prev := &(prevList.Samples[prevPos].Proc)
			pid := curList.Samples[curPos].Pid

			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &ProcSample{}
			}
			deltaMap[pid] = &ProcSample{}
			delta := &(deltaMap[pid].Proc)

			delta.CaptureTime = cur.CaptureTime
			duration := float64(cur.CaptureTime.Sub(prev.CaptureTime) / time.Millisecond)
			scale := float64(interval) / duration

			sum := &(sumMap[pid].Proc)
			sum.CaptureTime = cur.CaptureTime
			delta.Utime = ScaledSub(cur.Utime, prev.Utime, scale)
			sum.Utime += SafeSub(cur.Utime, prev.Utime)
			delta.Stime = ScaledSub(cur.Stime, prev.Stime, scale)
			sum.Stime += SafeSub(cur.Stime, prev.Stime)
			delta.Cutime = ScaledSub(cur.Cutime, prev.Cutime, scale)
			sum.Cutime += SafeSub(cur.Cutime, prev.Cutime)
			delta.Cstime = ScaledSub(cur.Cstime, prev.Cstime, scale)
			sum.Cstime += SafeSub(cur.Cstime, prev.Cstime)
			sum.Numthreads = cur.Numthreads
			sum.Rss = cur.Rss
			delta.Guesttime = ScaledSub(cur.Guesttime, prev.Guesttime, scale)
			sum.Guesttime += SafeSub(cur.Guesttime, prev.Guesttime)
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
