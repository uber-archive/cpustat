package main

import (
	"log"
	"strings"

	"github.com/codahale/hdrhistogram"
)

// scheduler stats by task/thread from /proc/[pid]/task/[tid]/sched
type schedStats struct {
	vruntime              float64
	execRuntime           float64
	waitSum               float64
	waitCount             uint64
	iowaitSum             float64
	iowaitCount           uint64
	nrSwitches            uint64
	nrVoluntarySwitches   uint64
	nrInvoluntarySwitches uint64
	clockDelta            uint64
}

type schedStatsMap map[int]*schedStats

func schedReaderPids(pidlist pidlist, conn *NLConn) schedStatsMap {
	ret := make(schedStatsMap)
	for _, pid := range pidlist {
		stats, err := lookupPid(conn, pid)
		if err != nil {
			continue
		}

		pidSum := schedStats{}
		pidSum.waitSum = float64(stats.CPUDelayTotal)
		pidSum.waitCount = stats.CPUCount
		pidSum.iowaitSum = float64(stats.BlkIODelayTotal)
		pidSum.iowaitCount = stats.BlkIOCount
		pidSum.nrVoluntarySwitches = stats.Nvcsw
		pidSum.nrInvoluntarySwitches = stats.Nivcsw
		ret[pid] = &pidSum
	}
	return ret
}

// this uses safeSub from stats.go
func schedRecord(curMap, prevMap, sumMap schedStatsMap) {
	for pid, cur := range curMap {
		if prev, ok := prevMap[pid]; ok == true {
			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &schedStats{}
			}
			sum := sumMap[pid]
			sum.vruntime += safeSubFloat(cur.vruntime, prev.vruntime)
			sum.execRuntime += safeSubFloat(cur.execRuntime, prev.execRuntime)
			sum.waitSum += safeSubFloat(cur.waitSum, prev.waitSum)
			sum.waitCount += safeSub(cur.waitCount, prev.waitCount)
			sum.iowaitSum += safeSubFloat(cur.iowaitSum, prev.iowaitSum)
			sum.iowaitCount += safeSub(cur.iowaitCount, prev.iowaitCount)
			sum.nrSwitches += safeSub(cur.nrSwitches, prev.nrSwitches)
			sum.nrVoluntarySwitches += safeSub(cur.nrVoluntarySwitches, prev.nrVoluntarySwitches)
			sum.nrInvoluntarySwitches += safeSub(cur.nrInvoluntarySwitches, prev.nrInvoluntarySwitches)
			sum.clockDelta += safeSub(cur.clockDelta, prev.clockDelta)
		}
	}
}

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
