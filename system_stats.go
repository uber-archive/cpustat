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
package main

import (
	"log"
	"strings"
	"time"

	"github.com/codahale/hdrhistogram"
)

type systemStats struct {
	captureTime  time.Time
	prevTime     time.Time
	usr          uint64
	nice         uint64
	sys          uint64
	idle         uint64
	iowait       uint64
	irq          uint64
	softirq      uint64
	steal        uint64
	guest        uint64
	guestNice    uint64
	ctxt         uint64
	procsTotal   uint64
	procsRunning uint64
	procsBlocked uint64
}

type systemStatsHist struct {
	usr          *hdrhistogram.Histogram
	nice         *hdrhistogram.Histogram
	sys          *hdrhistogram.Histogram
	idle         *hdrhistogram.Histogram
	iowait       *hdrhistogram.Histogram
	ctxt         *hdrhistogram.Histogram
	procsTotal   *hdrhistogram.Histogram
	procsRunning *hdrhistogram.Histogram
	procsBlocked *hdrhistogram.Histogram
}

func systemStatsReader() *systemStats {
	lines, err := readFileLines("/proc/stat")
	if err != nil {
		log.Fatal("reading /proc/stat: ", err)
	}

	cur := systemStats{}

	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), " ")
		switch parts[0] {
		case "cpu":
			cur.captureTime = time.Now()
			cur.prevTime = time.Time{}

			parts = parts[1:] // global cpu line has an extra space for some human somewhere
			cur.usr = readUInt(parts[1])
			cur.nice = readUInt(parts[2])
			cur.sys = readUInt(parts[3])
			cur.idle = readUInt(parts[4])
			cur.iowait = readUInt(parts[5])
			cur.irq = readUInt(parts[6])
			cur.softirq = readUInt(parts[7])
			cur.steal = readUInt(parts[8])
			cur.guest = readUInt(parts[9])
			cur.guestNice = readUInt(parts[10])
		case "ctxt":
			cur.ctxt = readUInt(parts[1])
		case "processes":
			cur.procsTotal = readUInt(parts[1])
		case "procs_running":
			cur.procsRunning = readUInt(parts[1])
		case "procs_blocked":
			cur.procsBlocked = readUInt(parts[1])
		default:
			continue
		}
	}

	return &cur
}

func systemStatsRecord(interval int, cur, prev, sum *systemStats, hist *systemStatsHist) *systemStats {
	delta := &systemStats{}

	sum.captureTime = cur.captureTime
	delta.captureTime = cur.captureTime
	delta.prevTime = prev.captureTime
	duration := float64(delta.captureTime.Sub(delta.prevTime) / time.Millisecond)
	scale := float64(interval) / duration

	delta.usr = scaledSub(cur.usr, prev.usr, scale)
	sum.usr += safeSub(cur.usr, prev.usr)
	delta.nice = scaledSub(cur.nice, prev.nice, scale)
	sum.nice += safeSub(cur.nice, prev.nice)
	delta.sys = scaledSub(cur.sys, prev.sys, scale)
	sum.sys += safeSub(cur.sys, prev.sys)
	delta.idle = scaledSub(cur.idle, prev.idle, scale)
	sum.idle += safeSub(cur.idle, prev.idle)
	delta.iowait = scaledSub(cur.iowait, prev.iowait, scale)
	sum.iowait += safeSub(cur.iowait, prev.iowait)
	delta.irq = scaledSub(cur.irq, prev.irq, scale)
	sum.irq += safeSub(cur.irq, prev.irq)
	delta.softirq = scaledSub(cur.softirq, prev.softirq, scale)
	sum.softirq += safeSub(cur.softirq, prev.softirq)
	delta.steal = scaledSub(cur.steal, prev.steal, scale)
	sum.steal += safeSub(cur.steal, prev.steal)
	delta.guest = scaledSub(cur.guest, prev.guest, scale)
	sum.guest += safeSub(cur.guest, prev.guest)
	delta.guestNice = scaledSub(cur.guestNice, prev.guestNice, scale)
	sum.guestNice += safeSub(cur.guestNice, prev.guestNice)
	delta.ctxt = scaledSub(cur.ctxt, prev.ctxt, scale)
	sum.ctxt += safeSub(cur.ctxt, prev.ctxt)
	delta.procsTotal = scaledSub(cur.procsTotal, prev.procsTotal, scale)
	sum.procsTotal += safeSub(cur.procsTotal, prev.procsTotal)
	sum.procsRunning = cur.procsRunning
	sum.procsBlocked = cur.procsBlocked

	if hist.usr == nil {
		hist.usr = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.nice = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.sys = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.idle = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.iowait = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.ctxt = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.procsTotal = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.procsRunning = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.procsBlocked = hdrhistogram.New(histMin, histMax, histSigFigs)
	}

	hist.usr.RecordValue(int64(delta.usr))
	hist.nice.RecordValue(int64(delta.nice))
	hist.sys.RecordValue(int64(delta.sys))
	hist.idle.RecordValue(int64(delta.idle))
	hist.iowait.RecordValue(int64(delta.iowait))
	hist.ctxt.RecordValue(int64(delta.ctxt))
	hist.procsTotal.RecordValue(int64(delta.procsTotal))
	hist.procsRunning.RecordValue(int64(cur.procsRunning))
	hist.procsBlocked.RecordValue(int64(cur.procsBlocked))

	return delta
}
