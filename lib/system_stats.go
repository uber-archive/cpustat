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

// data from /proc/stat about the entire system
// This data appears to be updated somewhat frequently
package cpustat

import (
	"fmt"
	"strings"
	"time"
)

// make this a package var so tests or users can change it
var StatsPath = "/proc/stat"

type SystemStats struct {
	CaptureTime  time.Time
	Usr          uint64
	Nice         uint64
	Sys          uint64
	Idle         uint64
	Iowait       uint64
	Irq          uint64
	Softirq      uint64
	Steal        uint64
	Guest        uint64
	GuestNice    uint64
	Ctxt         uint64
	ProcsTotal   uint64
	ProcsRunning uint64
	ProcsBlocked uint64
}

func SystemStatsReader(cur *SystemStats) error {
	lines, err := ReadFileLines(StatsPath)
	if err != nil {
		return fmt.Errorf("reading %s: %s", StatsPath, err)
	}

	if len(lines) <= 1 {
		return fmt.Errorf("reading %s: empty file read", StatsPath)
	}

	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), " ")
		switch parts[0] {
		case "cpu":
			cur.CaptureTime = time.Now()

			parts = parts[1:] // global cpu line has an extra space for some human somewhere
			cur.Usr = ReadUInt(parts[1])
			cur.Nice = ReadUInt(parts[2])
			cur.Sys = ReadUInt(parts[3])
			cur.Idle = ReadUInt(parts[4])
			cur.Iowait = ReadUInt(parts[5])
			cur.Irq = ReadUInt(parts[6])
			cur.Softirq = ReadUInt(parts[7])
			cur.Steal = ReadUInt(parts[8])
			cur.Guest = ReadUInt(parts[9])
			// Linux 2.6.33 introduced guestNice, just leave it 0 if it's not there
			if len(parts) == 11 {
				cur.GuestNice = ReadUInt(parts[10])
			}
		case "ctxt":
			cur.Ctxt = ReadUInt(parts[1])
		case "processes":
			cur.ProcsTotal = ReadUInt(parts[1])
		case "procs_running":
			cur.ProcsRunning = ReadUInt(parts[1])
		case "procs_blocked":
			cur.ProcsBlocked = ReadUInt(parts[1])
		default:
			continue
		}
	}

	return nil
}

func SystemStatsRecord(interval uint32, cur, prev, sum *SystemStats) *SystemStats {
	delta := &SystemStats{}

	sum.CaptureTime = cur.CaptureTime
	delta.CaptureTime = cur.CaptureTime
	duration := float64(cur.CaptureTime.Sub(prev.CaptureTime) / time.Millisecond)
	scale := float64(interval) / duration

	delta.Usr = ScaledSub(cur.Usr, prev.Usr, scale)
	sum.Usr += SafeSub(cur.Usr, prev.Usr)
	delta.Nice = ScaledSub(cur.Nice, prev.Nice, scale)
	sum.Nice += SafeSub(cur.Nice, prev.Nice)
	delta.Sys = ScaledSub(cur.Sys, prev.Sys, scale)
	sum.Sys += SafeSub(cur.Sys, prev.Sys)
	delta.Idle = ScaledSub(cur.Idle, prev.Idle, scale)
	sum.Idle += SafeSub(cur.Idle, prev.Idle)
	delta.Iowait = ScaledSub(cur.Iowait, prev.Iowait, scale)
	sum.Iowait += SafeSub(cur.Iowait, prev.Iowait)
	delta.Irq = ScaledSub(cur.Irq, prev.Irq, scale)
	sum.Irq += SafeSub(cur.Irq, prev.Irq)
	delta.Softirq = ScaledSub(cur.Softirq, prev.Softirq, scale)
	sum.Softirq += SafeSub(cur.Softirq, prev.Softirq)
	delta.Steal = ScaledSub(cur.Steal, prev.Steal, scale)
	sum.Steal += SafeSub(cur.Steal, prev.Steal)
	delta.Guest = ScaledSub(cur.Guest, prev.Guest, scale)
	sum.Guest += SafeSub(cur.Guest, prev.Guest)
	delta.GuestNice = ScaledSub(cur.GuestNice, prev.GuestNice, scale)
	sum.GuestNice += SafeSub(cur.GuestNice, prev.GuestNice)
	delta.Ctxt = ScaledSub(cur.Ctxt, prev.Ctxt, scale)
	sum.Ctxt += SafeSub(cur.Ctxt, prev.Ctxt)
	delta.ProcsTotal = ScaledSub(cur.ProcsTotal, prev.ProcsTotal, scale)
	sum.ProcsTotal += SafeSub(cur.ProcsTotal, prev.ProcsTotal)
	sum.ProcsRunning = cur.ProcsRunning
	delta.ProcsRunning = cur.ProcsRunning
	sum.ProcsBlocked = cur.ProcsBlocked
	delta.ProcsBlocked = cur.ProcsBlocked

	return delta
}
