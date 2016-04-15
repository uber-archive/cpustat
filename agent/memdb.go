package main

import (
	"sync"

	"github.com/uber-common/cpustat/lib"
)

type sampleBatch struct {
	Proc cpustat.ProcStatsMap
	Task cpustat.TaskStatsMap
	Sys  *cpustat.SystemStats
}

var dbData []sampleBatch
var dbLock sync.RWMutex
var dbSize uint32
var writePos uint32
var dbEntries uint32

func dbInit(newSize uint32) {
	if newSize < 1 {
		panic("db size must be larger than 0")
	}
	dbSize = newSize
	dbData = make([]sampleBatch, dbSize)
}

func dbCount() uint32 {
	dbLock.RLock()
	ret := dbEntries
	dbLock.RUnlock()
	return ret
}

func dbStats() (int, int, int) {
	var pcount, tcount, scount int

	readPos := int(writePos) - 1
	dbLock.RLock()
	for i := uint32(0); i < dbEntries; i++ {
		if readPos < 0 {
			readPos = int(dbSize) - 1
		}
		pcount += len(dbData[readPos].Proc)
		tcount += len(dbData[readPos].Task)
		scount++
		readPos--
	}
	dbLock.RUnlock()
	return pcount, tcount, scount
}

func writeSample(procMap cpustat.ProcStatsMap, taskMap cpustat.TaskStatsMap, sys *cpustat.SystemStats) {
	sample := sampleBatch{procMap, taskMap, sys}

	dbLock.Lock()
	dbData[writePos] = sample
	writePos++
	if writePos >= dbSize {
		writePos = 0
	}
	dbEntries++
	if dbEntries > dbSize {
		dbEntries = dbSize
	}
	dbLock.Unlock()
}

// TODO - figure out how expensive copying this is, because if the buffer wraps around
//        on us while the caller is holding these results, they could get overwritten.
func readSamples(n uint32) []sampleBatch {
	if n > dbEntries {
		n = dbEntries
	}
	ret := make([]sampleBatch, n)
	readPos := int(writePos) - 1
	dbLock.RLock()
	for i := n - 1; i >= 0; i-- {
		if readPos < 0 {
			readPos = int(dbSize) - 1
		}
		ret[i] = dbData[readPos]
		readPos--
	}
	dbLock.RUnlock()

	return ret
}
