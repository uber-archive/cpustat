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

// Use a histogram for each sample for each pid. This is really overkill, since all we use
// is min/max/avg. Maybe replace this wth something that more efficiently computes min/max/avg.
// Or else maybe make use fancy percentiles, since we have them.

package cpustat

import (
	"github.com/codahale/hdrhistogram"
)

const histMin = 0
const histMax = 100000000
const histSigFigs = 2

// These are somewhat expensive to track, so only maintain a hist for ones we use
type ProcStatsHist struct {
	Utime   *hdrhistogram.Histogram
	Stime   *hdrhistogram.Histogram
	Ustime  *hdrhistogram.Histogram // utime + stime
	Cutime  *hdrhistogram.Histogram
	Cstime  *hdrhistogram.Histogram
	Custime *hdrhistogram.Histogram // cutime + cstime
}

type ProcStatsHistMap map[int]*ProcStatsHist

func UpdateProcStatsHist(histMap ProcStatsHistMap, deltaMap ProcSampleMap) {
	for pid, deltaSample := range deltaMap {
		if _, ok := histMap[pid]; ok != true {
			histMap[pid] = NewProcStatsHist()
		}
		hist := histMap[pid]

		delta := &(deltaSample.Proc)
		hist.Utime.RecordValue(int64(delta.Utime))
		hist.Stime.RecordValue(int64(delta.Stime))
		hist.Ustime.RecordValue(int64(delta.Utime + delta.Stime))
		hist.Cutime.RecordValue(int64(delta.Cutime))
		hist.Cstime.RecordValue(int64(delta.Cstime))
		hist.Custime.RecordValue(int64(delta.Cutime + delta.Cstime))
	}
}

func NewProcStatsHist() *ProcStatsHist {
	return &ProcStatsHist{
		hdrhistogram.New(histMin, histMax, histSigFigs),
		hdrhistogram.New(histMin, histMax, histSigFigs),
		hdrhistogram.New(histMin, histMax, histSigFigs),
		hdrhistogram.New(histMin, histMax, histSigFigs),
		hdrhistogram.New(histMin, histMax, histSigFigs),
		hdrhistogram.New(histMin, histMax, histSigFigs),
	}
}

type TaskStatsHist struct {
	Cpudelay *hdrhistogram.Histogram
	Iowait   *hdrhistogram.Histogram
	Swap     *hdrhistogram.Histogram
}

type TaskStatsHistMap map[int]*TaskStatsHist

func UpdateTaskStatsHist(histMap TaskStatsHistMap, deltaMap ProcSampleMap) {
	for pid, deltaSample := range deltaMap {
		if _, ok := histMap[pid]; ok != true {
			histMap[pid] = NewTaskStatsHist()
		}
		hist := histMap[pid]
		delta := &(deltaSample.Task)

		hist.Cpudelay.RecordValue(int64(delta.Cpudelaytotal))
		hist.Iowait.RecordValue(int64(delta.Blkiodelaytotal))
		hist.Swap.RecordValue(int64(delta.Swapindelaytotal))
	}
}

func NewTaskStatsHist() *TaskStatsHist {
	return &TaskStatsHist{
		hdrhistogram.New(histMin, histMax, histSigFigs),
		hdrhistogram.New(histMin, histMax, histSigFigs),
		hdrhistogram.New(histMin, histMax, histSigFigs),
	}
}

type SystemStatsHist struct {
	Usr          *hdrhistogram.Histogram
	Nice         *hdrhistogram.Histogram
	Sys          *hdrhistogram.Histogram
	Idle         *hdrhistogram.Histogram
	Iowait       *hdrhistogram.Histogram
	ProcsTotal   *hdrhistogram.Histogram
	ProcsRunning *hdrhistogram.Histogram
	ProcsBlocked *hdrhistogram.Histogram
}

func UpdateSysStatsHist(hist *SystemStatsHist, delta *SystemStats) {
	hist.Usr.RecordValue(int64(delta.Usr))
	hist.Nice.RecordValue(int64(delta.Nice))
	hist.Sys.RecordValue(int64(delta.Sys))
	hist.Idle.RecordValue(int64(delta.Idle))
	hist.Iowait.RecordValue(int64(delta.Iowait))
	hist.ProcsTotal.RecordValue(int64(delta.ProcsTotal))
	hist.ProcsRunning.RecordValue(int64(delta.ProcsRunning))
	hist.ProcsBlocked.RecordValue(int64(delta.ProcsBlocked))
}

func NewSysStatsHist() *SystemStatsHist {
	hist := SystemStatsHist{}
	hist.Usr = hdrhistogram.New(histMin, histMax, histSigFigs)
	hist.Nice = hdrhistogram.New(histMin, histMax, histSigFigs)
	hist.Sys = hdrhistogram.New(histMin, histMax, histSigFigs)
	hist.Idle = hdrhistogram.New(histMin, histMax, histSigFigs)
	hist.Iowait = hdrhistogram.New(histMin, histMax, histSigFigs)
	hist.ProcsTotal = hdrhistogram.New(histMin, histMax, histSigFigs)
	hist.ProcsRunning = hdrhistogram.New(histMin, histMax, histSigFigs)
	hist.ProcsBlocked = hdrhistogram.New(histMin, histMax, histSigFigs)

	return &hist
}
