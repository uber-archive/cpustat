package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
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

// from pidlist, find all tasks, read and summarize their stats
func schedReaderPids(pidlist pidlist) schedStatsMap {
	ret := make(schedStatsMap)
	for _, pid := range pidlist {
		var taskDir *os.File
		var taskNames []string
		var err error

		if taskDir, err = os.Open(fmt.Sprintf("/proc/%d/task", pid)); err != nil {
			continue
		}
		if taskNames, err = taskDir.Readdirnames(0); err != nil {
			log.Fatal("Readdirnames: ", err)
		}
		var taskid int
		pidSum := schedStats{}
		for _, taskName := range taskNames {
			// skip non-numbers that might happen to be in this dir
			if taskid, err = strconv.Atoi(taskName); err != nil {
				continue
			}
			lines, err := readFileLines(fmt.Sprintf("/proc/%d/task/%d/sched", pid, taskid))
			// proc or thread could have exited between when we scanned the dir and now
			if err != nil {
				continue
			}
			for _, line := range lines[2:] {
				parts := strings.Split(line, ":")
				if len(parts) != 2 {
					continue
				}
				parts[0] = strings.TrimSpace(parts[0])
				parts[1] = strings.TrimSpace(parts[1])
				switch parts[0] {
				case "se.vruntime":
					pidSum.vruntime += readFloat(parts[1])
				case "se.sum_exec_runtime":
					pidSum.execRuntime += readFloat(parts[1])
				case "se.statistics.wait_sum":
					pidSum.waitSum += readFloat(parts[1])
				case "se.statistics.wait_count":
					pidSum.waitSum += readFloat(parts[1])
				case "se.statistics.iowait_sum":
					pidSum.iowaitSum += readFloat(parts[1])
				case "se.statistics.iowait_count":
					pidSum.waitCount += readUInt(parts[1])
				case "nr_switches":
					pidSum.nrSwitches += readUInt(parts[1])
				case "nr_voluntary_switches":
					pidSum.nrVoluntarySwitches += readUInt(parts[1])
				case "nr_involuntary_switches":
					pidSum.nrInvoluntarySwitches += readUInt(parts[1])
				case "clock-delta":
					pidSum.clockDelta = readUInt(parts[1])
				}
			}
			ret[pid] = &pidSum
		}
	}
	return ret
}

func schedRecord(curMap, prevMap, sumMap schedStatsMap) {
	for pid, cur := range curMap {
		if prev, ok := prevMap[pid]; ok == true {
			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &schedStats{}
			}
			sum := sumMap[pid]
			sum.vruntime += (cur.vruntime - prev.vruntime)
			sum.execRuntime += (cur.execRuntime - prev.execRuntime)
			sum.waitSum += (cur.waitSum - prev.waitSum)
			sum.waitCount += (cur.waitCount - prev.waitCount)
			sum.iowaitSum += (cur.iowaitSum - prev.iowaitSum)
			sum.iowaitCount += (cur.iowaitCount - prev.iowaitCount)
			sum.nrSwitches += (cur.nrSwitches - prev.nrSwitches)
			sum.nrVoluntarySwitches += (cur.nrVoluntarySwitches - prev.nrVoluntarySwitches)
			sum.nrInvoluntarySwitches += (cur.nrInvoluntarySwitches - prev.nrInvoluntarySwitches)
			sum.clockDelta += (cur.clockDelta - prev.clockDelta)
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
