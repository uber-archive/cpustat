package main

import (
	"sync"

	"github.com/uber-common/cpustat/lib"
)

type dbEntry struct {
	Proc cpustat.ProcSampleList
	Sys  cpustat.SystemStats
}

type MemDB struct {
	dbData    []dbEntry
	dbLock    sync.RWMutex
	dbMaxSize uint32
	writePos  uint32
	dbEntries uint32
}

func (m *MemDB) Init(newSize, maxProcsToScan uint32) {
	if newSize < 1 {
		panic("db size must be larger than 0")
	}
	m.dbMaxSize = newSize
	m.dbData = make([]dbEntry, m.dbMaxSize)
	for pos := range m.dbData {
		m.dbData[pos] = dbEntry{
			cpustat.ProcSampleList{},
			cpustat.SystemStats{},
		}
		m.dbData[pos].Proc.Samples = make([]cpustat.ProcSample, maxProcsToScan)
	}
}

func (m *MemDB) DBCount() uint32 {
	m.dbLock.RLock()
	ret := m.dbEntries
	m.dbLock.RUnlock()
	return ret
}

func (m *MemDB) DBStats() (uint32, uint32) {
	var pcount, scount uint32

	readPos := int(m.writePos) - 1
	m.dbLock.RLock()
	for i := uint32(0); i < m.dbEntries; i++ {
		if readPos < 0 {
			readPos = int(m.dbMaxSize) - 1
		}
		pcount += m.dbData[readPos].Proc.Len
		scount++
		readPos--
	}
	m.dbLock.RUnlock()
	return pcount, scount
}

func (m *MemDB) WriteSample(procList cpustat.ProcSampleList, sys *cpustat.SystemStats) {
	sample := dbEntry{procList, *sys}

	m.dbLock.Lock()
	m.dbData[m.writePos] = sample
	m.writePos++
	if m.writePos >= m.dbMaxSize {
		m.writePos = 0
	}
	m.dbEntries++
	if m.dbEntries > m.dbMaxSize {
		m.dbEntries = m.dbMaxSize
	}
	m.dbLock.Unlock()
}

func (m *MemDB) ReserveSample() *dbEntry {
	m.dbLock.Lock()

	return &m.dbData[m.writePos]
}

func (m *MemDB) ReleaseSample() {
	m.writePos++
	if m.writePos >= m.dbMaxSize {
		m.writePos = 0
	}
	m.dbEntries++
	if m.dbEntries > m.dbMaxSize {
		m.dbEntries = m.dbMaxSize
	}

	m.dbLock.Unlock()
}

// TODO - figure out how expensive copying this is, because if the buffer wraps around
//        on us while the caller is holding these results, they could get overwritten.
func (m *MemDB) ReadSamples(n uint32) []dbEntry {
	if n > m.dbEntries {
		n = m.dbEntries
	}
	ret := make([]dbEntry, n)
	readPos := int(m.writePos) - 1
	m.dbLock.RLock()
	for i := int(n) - 1; i >= 0; i-- {
		if readPos < 0 {
			readPos = int(m.dbMaxSize) - 1
		}
		ret[i] = m.dbData[readPos]
		readPos--
	}
	m.dbLock.RUnlock()

	return ret
}
