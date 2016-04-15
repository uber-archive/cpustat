package main

import (
	"fmt"
	"sync"

	"github.com/uber-common/cpustat/lib"
)

type dbEntry struct {
	cpustat.ProcSampleList
	cpustat.SystemStats
}

var dbData []dbEntry
var dbLock sync.RWMutex
var dbMaxSize uint32
var writePos uint32
var dbEntries uint32

func dbInit(newSize uint32) {
	if newSize < 1 {
		panic("db size must be larger than 0")
	}
	dbMaxSize = newSize
	dbData = make([]dbEntry, dbMaxSize)
}

func dbCount() uint32 {
	dbLock.RLock()
	ret := dbEntries
	dbLock.RUnlock()
	return ret
}

func dbStats() (int, int) {
	var pcount, scount int

	readPos := int(writePos) - 1
	dbLock.RLock()
	for i := uint32(0); i < dbEntries; i++ {
		if readPos < 0 {
			readPos = int(dbMaxSize) - 1
		}
		pcount += len(dbData[readPos].ProcSampleList)
		scount++
		readPos--
	}
	dbLock.RUnlock()
	return pcount, scount
}

func writeSample(procList cpustat.ProcSampleList, sys *cpustat.SystemStats) {
	sample := dbEntry{procList, *sys}

	dbLock.Lock()
	dbData[writePos] = sample
	writePos++
	if writePos >= dbMaxSize {
		writePos = 0
	}
	dbEntries++
	if dbEntries > dbMaxSize {
		dbEntries = dbMaxSize
	}
	dbLock.Unlock()
}

// TODO - figure out how expensive copying this is, because if the buffer wraps around
//        on us while the caller is holding these results, they could get overwritten.
func readSamples(n uint32) []dbEntry {
	if n > dbEntries {
		n = dbEntries
	}
	ret := make([]dbEntry, n)
	readPos := int(writePos) - 1
	dbLock.RLock()
	for i := int(n) - 1; i >= 0; i-- {
		if readPos < 0 {
			readPos = int(dbMaxSize) - 1
		}
		fmt.Println(i, readPos, n, len(ret), len(dbData))
		ret[i] =
			dbData[readPos]
		readPos--
	}
	dbLock.RUnlock()

	return ret
}
