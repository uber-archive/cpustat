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

// note that this is no longer used.
// The data from /proc seems less consistent that taskstats. I could be imagining this though.
// If it turns out that I am imagining it, we should go back to using this, which is easy to understand.

package main

import (
	"log"
	"strings"

	"github.com/codahale/hdrhistogram"
)

// system-waitCount scheduler waitCount by CPU from /proc/schedstat
type schedStatsCPU struct {
	schedYieldCalls  uint64 // 1
	scheduleCalls    uint64 // 3
	scheduleProcIdle uint64 // 4
	wakeUpCalls      uint64 // 5
	wakeUpCallsLocal uint64 // 6
	timeRunning      uint64 // 7
	timeWaiting      uint64 // 8
	timeSlices       uint64 // 9
}

type schedStatsCPUHist struct {
	schedYieldCalls  *hdrhistogram.Histogram
	scheduleCalls    *hdrhistogram.Histogram
	scheduleProcIdle *hdrhistogram.Histogram
	wakeUpCalls      *hdrhistogram.Histogram
	wakeUpCallsLocal *hdrhistogram.Histogram
	timeRunning      *hdrhistogram.Histogram
	timeWaiting      *hdrhistogram.Histogram
	timeSlices       *hdrhistogram.Histogram
}

func schedReaderCPU() *schedStatsCPU {
	lines, err := readFileLines("/proc/schedstat")
	if err != nil {
		log.Fatal("reading /proc/stat: ", err)
	}

	cur := schedStatsCPU{}

	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), " ")
		if strings.Index(parts[0], "cpu") == 0 {
			cur.schedYieldCalls += readUInt(parts[1])
			cur.scheduleCalls += readUInt(parts[3])
			cur.scheduleProcIdle += readUInt(parts[4])
			cur.wakeUpCalls += readUInt(parts[5])
			cur.wakeUpCallsLocal += readUInt(parts[6])
			cur.timeRunning += readUInt(parts[7])
			cur.timeWaiting += readUInt(parts[8])
			cur.timeSlices += readUInt(parts[9])
		}
		// skip all the domain lines, maybe add them at some point
	}

	return &cur
}

func schedCPURecord(cur, prev, sum *schedStatsCPU, hist *schedStatsCPUHist) {
	sum.schedYieldCalls += (cur.schedYieldCalls - prev.schedYieldCalls)
	sum.scheduleCalls += (cur.scheduleCalls - prev.scheduleCalls)
	sum.scheduleProcIdle += (cur.scheduleProcIdle - prev.scheduleProcIdle)
	sum.wakeUpCalls += (cur.wakeUpCalls - prev.wakeUpCalls)
	sum.wakeUpCallsLocal += (cur.wakeUpCallsLocal - prev.wakeUpCallsLocal)
	sum.timeRunning += (cur.timeRunning - prev.timeRunning)
	sum.timeWaiting += (cur.timeWaiting - prev.timeWaiting)
	sum.timeSlices += (cur.timeSlices - prev.timeSlices)

	if hist.schedYieldCalls == nil {
		hist.schedYieldCalls = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.scheduleCalls = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.scheduleProcIdle = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.wakeUpCalls = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.wakeUpCallsLocal = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.timeRunning = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.timeWaiting = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.timeSlices = hdrhistogram.New(histMin, histMax, histSigFigs)
	}

	hist.schedYieldCalls.RecordValue(int64(cur.schedYieldCalls - prev.schedYieldCalls))
	hist.scheduleCalls.RecordValue(int64(cur.scheduleCalls - prev.scheduleCalls))
	hist.scheduleProcIdle.RecordValue(int64(cur.scheduleProcIdle - prev.scheduleProcIdle))
	hist.wakeUpCalls.RecordValue(int64(cur.wakeUpCalls - prev.wakeUpCalls))
	hist.wakeUpCallsLocal.RecordValue(int64(cur.wakeUpCallsLocal - prev.wakeUpCallsLocal))
	hist.timeRunning.RecordValue(int64(cur.timeRunning - prev.timeRunning))
	hist.timeWaiting.RecordValue(int64(cur.timeWaiting - prev.timeWaiting))
	hist.timeSlices.RecordValue(int64(cur.timeSlices - prev.timeSlices))
}
