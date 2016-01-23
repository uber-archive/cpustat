package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/codahale/hdrhistogram"
)

// nearly all of the values from /proc/[pid]/stat
type procStats struct {
	pid                 uint64
	comm                string
	state               string
	ppid                uint64
	pgrp                int64
	session             int64
	ttyNr               int64
	tpgid               int64
	flags               uint64
	minflt              uint64
	cminflt             uint64
	majflt              uint64
	cmajflt             uint64
	utime               uint64
	stime               uint64
	cutime              uint64
	cstime              uint64
	priority            int64
	nice                int64
	numThreads          uint64
	startTime           uint64
	vsize               uint64
	rss                 uint64
	rsslim              uint64
	processor           uint64
	rtPriority          uint64
	policy              uint64
	delayacctBlkioTicks uint64
	guestTime           uint64
	cguestTime          uint64
}

type procStatsHist struct {
	minflt              *hdrhistogram.Histogram
	cminflt             *hdrhistogram.Histogram
	majflt              *hdrhistogram.Histogram
	cmajflt             *hdrhistogram.Histogram
	utime               *hdrhistogram.Histogram
	stime               *hdrhistogram.Histogram
	cutime              *hdrhistogram.Histogram
	cstime              *hdrhistogram.Histogram
	nice                *hdrhistogram.Histogram
	delayacctBlkioTicks *hdrhistogram.Histogram
	guestTime           *hdrhistogram.Histogram
	cguestTime          *hdrhistogram.Histogram
}

type procStatsMap map[int]*procStats
type procStatsHistMap map[int]*procStatsHist

func procPidStatSplit(line string) []string {
	line = strings.TrimSpace(line)
	parts := make([]string, 52)

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
				parts[partnum] = line[start:strpos]
				partnum++
				start = strpos
				inword = false
			}
		} else {
			if line[strpos] == open {
				groupchar = close
				inword = true
				start = strpos
			} else if line[strpos] != space {
				groupchar = space
				inword = true
				start = strpos
			}
		}
	}

	if inword {
		parts[partnum] = line[start:strpos]
	}

	return parts
}

func statReader(pids pidlist) procStatsMap {
	cur := make(procStatsMap)

	for _, pid := range pids {
		lines, err := readFileLines(fmt.Sprintf("/proc/%d/stat", pid))
		// pid could have exited between when we scanned the dir and now
		if err != nil {
			continue
		}

		// this file should only be one line total
		parts := procPidStatSplit(lines[0])

		stat := procStats{
			readUInt(parts[0]),  // pid
			parts[1],            //comm
			parts[2],            // state
			readUInt(parts[3]),  // ppid
			readInt(parts[4]),   // pgrp
			readInt(parts[5]),   // session
			readInt(parts[6]),   // tty_nr
			readInt(parts[7]),   // tpgid
			readUInt(parts[8]),  // flags
			readUInt(parts[9]),  // minflt
			readUInt(parts[10]), // cminflt
			readUInt(parts[11]), // majflt
			readUInt(parts[12]), // cmajflt
			readUInt(parts[13]), // utime
			readUInt(parts[14]), // stime
			readUInt(parts[15]), // cutime
			readUInt(parts[16]), // cstime
			readInt(parts[17]),  // priority
			readInt(parts[18]),  // nice
			readUInt(parts[19]), // num_threads
			// itrealvalue - not maintained
			readUInt(parts[21]), // starttime
			readUInt(parts[22]), // vsize
			readUInt(parts[23]), // rss
			readUInt(parts[24]), // rsslim
			// bunch of stuff about memory addresses
			readUInt(parts[38]), // processor
			readUInt(parts[39]), // rt_priority
			readUInt(parts[40]), // policy
			readUInt(parts[41]), // delayacct_blkio_ticks
			readUInt(parts[42]), // guest_time
			readUInt(parts[43]), // cguest_time
		}
		cur[pid] = &stat
	}
	return cur
}

func statRecord(curMap, prevMap, sumMap procStatsMap, histMap procStatsHistMap) {
	for pid, cur := range curMap {
		if prev, ok := prevMap[pid]; ok == true {
			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &procStats{}
			}
			sum := sumMap[pid]
			sum.pid = cur.pid
			sum.comm = cur.comm
			sum.state = cur.state
			sum.ppid = cur.ppid
			sum.pgrp = cur.pgrp
			sum.session = cur.session
			sum.ttyNr = cur.ttyNr
			sum.tpgid = cur.tpgid
			sum.flags = cur.flags
			sum.minflt += (cur.minflt - prev.minflt)
			sum.cminflt += (cur.cminflt - prev.cminflt)
			sum.majflt += (cur.majflt - prev.majflt)
			sum.cmajflt += (cur.cmajflt - prev.cmajflt)
			sum.utime += (cur.utime - prev.utime)
			sum.stime += (cur.stime - prev.stime)
			sum.cutime += (cur.cutime - prev.cutime)
			sum.cstime += (cur.cstime - prev.cstime)
			sum.priority = cur.priority
			sum.nice = cur.nice
			sum.numThreads = cur.numThreads
			sum.startTime = cur.startTime
			sum.vsize = cur.vsize
			sum.rss = cur.rss
			sum.rsslim = cur.rsslim
			sum.processor = cur.processor
			sum.rtPriority = cur.rtPriority
			sum.policy = cur.policy
			sum.delayacctBlkioTicks += (cur.delayacctBlkioTicks - prev.delayacctBlkioTicks)
			sum.guestTime += (cur.guestTime - prev.guestTime)

			if sum.comm == "(uber-metrics)" {
				fmt.Println(sum.utime)
			}

			var hist *procStatsHist
			if hist, ok = histMap[pid]; ok != true {
				histMap[pid] = &procStatsHist{
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
				}
				hist = histMap[pid]
			}
			hist.minflt.RecordValue(int64(cur.minflt - prev.minflt))
			hist.cminflt.RecordValue(int64(cur.cminflt - prev.cminflt))
			hist.majflt.RecordValue(int64(cur.majflt - prev.majflt))
			hist.cmajflt.RecordValue(int64(cur.cmajflt - prev.cmajflt))
			hist.utime.RecordValue(int64(cur.utime - prev.utime))
			hist.stime.RecordValue(int64(cur.stime - prev.stime))
			hist.cutime.RecordValue(int64(cur.cutime - prev.cutime))
			hist.cstime.RecordValue(int64(cur.cstime - prev.cstime))
			hist.delayacctBlkioTicks.RecordValue(int64(cur.delayacctBlkioTicks - prev.delayacctBlkioTicks))
			hist.guestTime.RecordValue(int64(cur.guestTime - prev.guestTime))
		}
	}
}

// from /proc/stat
type systemStats struct {
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
	irq          *hdrhistogram.Histogram
	softirq      *hdrhistogram.Histogram
	steal        *hdrhistogram.Histogram
	guest        *hdrhistogram.Histogram
	guestNice    *hdrhistogram.Histogram
	ctxt         *hdrhistogram.Histogram
	procsTotal   *hdrhistogram.Histogram
	procsRunning *hdrhistogram.Histogram
	procsBlocked *hdrhistogram.Histogram
}

func statReaderGlobal() *systemStats {
	lines, err := readFileLines("/proc/stat")
	if err != nil {
		log.Fatal("reading /proc/stat: ", err)
	}

	cur := systemStats{}

	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), " ")
		switch parts[0] {
		case "cpu":
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

func statsRecordGlobal(cur, prev, sum *systemStats, hist *systemStatsHist) {
	sum.usr += (cur.usr - prev.usr)
	sum.nice += (cur.nice - prev.nice)
	sum.sys += (cur.sys - prev.sys)
	sum.idle += (cur.idle - prev.idle)
	sum.iowait += (cur.iowait - prev.iowait)
	sum.irq += (cur.irq - prev.irq)
	sum.softirq += (cur.softirq - prev.softirq)
	sum.steal += (cur.steal - prev.steal)
	sum.guest += (cur.guest - prev.guest)
	sum.guestNice += (cur.guestNice - prev.guestNice)
	sum.ctxt += (cur.ctxt - prev.ctxt)
	sum.procsTotal += (cur.procsTotal - prev.procsTotal)
	sum.procsRunning = cur.procsRunning
	sum.procsBlocked = cur.procsBlocked

	if hist.usr == nil {
		hist.usr = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.nice = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.sys = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.idle = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.iowait = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.irq = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.softirq = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.steal = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.guest = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.guestNice = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.ctxt = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.procsTotal = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.procsRunning = hdrhistogram.New(histMin, histMax, histSigFigs)
		hist.procsBlocked = hdrhistogram.New(histMin, histMax, histSigFigs)
	}

	hist.usr.RecordValue(int64(cur.usr - prev.usr))
	hist.nice.RecordValue(int64(cur.nice - prev.nice))
	hist.sys.RecordValue(int64(cur.sys - prev.sys))
	hist.idle.RecordValue(int64(cur.idle - prev.idle))
	hist.iowait.RecordValue(int64(cur.iowait - prev.iowait))
	hist.irq.RecordValue(int64(cur.irq - prev.irq))
	hist.softirq.RecordValue(int64(cur.softirq - prev.softirq))
	hist.steal.RecordValue(int64(cur.steal - prev.steal))
	hist.guest.RecordValue(int64(cur.guest - prev.guest))
	hist.guestNice.RecordValue(int64(cur.guestNice - prev.guestNice))
	hist.ctxt.RecordValue(int64(cur.ctxt - prev.ctxt))
	hist.procsTotal.RecordValue(int64(cur.procsTotal - prev.procsTotal))
	hist.procsRunning.RecordValue(int64(cur.procsRunning))
	hist.procsBlocked.RecordValue(int64(cur.procsBlocked))
}
