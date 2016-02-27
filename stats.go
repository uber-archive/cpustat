package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/codahale/hdrhistogram"
)

// nearly all of the values from /proc/[pid]/stat
type procStats struct {
	captureTime         time.Time
	prevTime            time.Time
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
	ustime              *hdrhistogram.Histogram // utime + stime
	cutime              *hdrhistogram.Histogram
	cstime              *hdrhistogram.Histogram
	custime             *hdrhistogram.Histogram // cutime + cstime
	nice                *hdrhistogram.Histogram
	delayacctBlkioTicks *hdrhistogram.Histogram
	guestTime           *hdrhistogram.Histogram
	cguestTime          *hdrhistogram.Histogram
}

type procStatsMap map[int]*procStats
type procStatsHistMap map[int]*procStatsHist

// you might think that we could split on space, but due to what can at best be called
// a shortcoming of the /proc/pid/stat format, the comm field can have spaces, parens, etc.
// and it is unescaped.
// This is a big paranoid, because even many common tools like htop do not handle this case
// well.
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
				strpos = strings.LastIndex(line, ")") - 1
				if strpos <= start { // if we can't parse this insane field, skip to the end
					strpos = len(line)
					inword = false
				}
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

func stripSpecial(r rune) rune {
	if r == '[' || r == ']' || r == '(' || r == ')' {
		return -1
	}
	return r
}

func statReader(pids pidlist, cmdNames cmdlineMap) procStatsMap {
	cur := make(procStatsMap)

	for _, pid := range pids {
		lines, err := readFileLines(fmt.Sprintf("/proc/%d/stat", pid))
		// pid could have exited between when we scanned the dir and now
		if err != nil {
			continue
		}

		// this format of this file is insane because comm can have split chars in it
		parts := procPidStatSplit(lines[0])

		stat := procStats{
			time.Now(), // this might be expensive. If so, can cache it. We only need 1ms resolution
			time.Time{},
			readUInt(parts[0]),                  // pid
			strings.Map(stripSpecial, parts[1]), // comm
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
		updateCmdline(cmdNames, pid, stat.comm)
	}
	return cur
}

// crude protection against rollover. This will miss the last portion of the previous sample
// before the overflow, but capturing that is complicated because of the various number types
// involved and their inconsistent documentation.
func safeSub(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return a - b
}

func safeSubFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return a - b
}

func scaledSub(cur, prev uint64, scale float64) uint64 {
	return uint64((float64(safeSub(cur, prev)) * scale) + 0.5)
}

func statRecord(interval int, curMap, prevMap, sumMap procStatsMap, histMap procStatsHistMap) procStatsMap {
	deltaMap := make(procStatsMap)

	for pid, cur := range curMap {
		if prev, ok := prevMap[pid]; ok == true {
			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &procStats{}
			}
			deltaMap[pid] = &procStats{}
			delta := deltaMap[pid]

			delta.captureTime = cur.captureTime
			delta.prevTime = prev.captureTime
			duration := float64(delta.captureTime.Sub(delta.prevTime) / time.Millisecond)
			scale := float64(interval) / duration

			sum := sumMap[pid]
			sum.captureTime = cur.captureTime
			sum.pid = cur.pid
			sum.comm = cur.comm
			sum.state = cur.state
			sum.ppid = cur.ppid
			sum.pgrp = cur.pgrp
			sum.session = cur.session
			sum.ttyNr = cur.ttyNr
			sum.tpgid = cur.tpgid
			sum.flags = cur.flags
			delta.minflt = scaledSub(cur.minflt, prev.minflt, scale)
			sum.minflt += safeSub(cur.minflt, prev.minflt)
			delta.cminflt = scaledSub(cur.cminflt, prev.cminflt, scale)
			sum.cminflt += safeSub(cur.cminflt, prev.cminflt)
			delta.majflt = scaledSub(cur.majflt, prev.majflt, scale)
			sum.majflt += safeSub(cur.majflt, prev.majflt)
			delta.cmajflt = scaledSub(cur.cmajflt, prev.cmajflt, scale)
			sum.cmajflt += safeSub(cur.cmajflt, prev.cmajflt)
			delta.utime = scaledSub(cur.utime, prev.utime, scale)
			sum.utime += safeSub(cur.utime, prev.utime)
			delta.stime = scaledSub(cur.stime, prev.stime, scale)
			sum.stime += safeSub(cur.stime, prev.stime)
			delta.cutime = scaledSub(cur.cutime, prev.cutime, scale)
			sum.cutime += safeSub(cur.cutime, prev.cutime)
			delta.cstime = scaledSub(cur.cstime, prev.cstime, scale)
			sum.cstime += safeSub(cur.cstime, prev.cstime)
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
			delta.delayacctBlkioTicks = scaledSub(cur.delayacctBlkioTicks, prev.delayacctBlkioTicks, scale)
			sum.delayacctBlkioTicks += safeSub(cur.delayacctBlkioTicks, prev.delayacctBlkioTicks)
			delta.guestTime = scaledSub(cur.guestTime, prev.guestTime, scale)
			sum.guestTime += safeSub(cur.guestTime, prev.guestTime)

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
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
				}
				hist = histMap[pid]
			}
			hist.minflt.RecordValue(int64(delta.minflt))
			hist.cminflt.RecordValue(int64(delta.cminflt))
			hist.majflt.RecordValue(int64(delta.majflt))
			hist.cmajflt.RecordValue(int64(delta.cmajflt))
			hist.utime.RecordValue(int64(delta.utime))
			hist.stime.RecordValue(int64(delta.stime))
			hist.ustime.RecordValue(int64(delta.utime + delta.stime))
			hist.cutime.RecordValue(int64(delta.cutime))
			hist.cstime.RecordValue(int64(delta.cstime))
			hist.custime.RecordValue(int64(delta.cutime + delta.cstime))
			hist.delayacctBlkioTicks.RecordValue(int64(delta.delayacctBlkioTicks))
			hist.guestTime.RecordValue(int64(delta.guestTime))
		}
	}

	return deltaMap
}

// from /proc/stat
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

func statsRecordGlobal(interval int, cur, prev, sum *systemStats, hist *systemStatsHist) *systemStats {
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

	hist.usr.RecordValue(int64(delta.usr))
	hist.nice.RecordValue(int64(delta.nice))
	hist.sys.RecordValue(int64(delta.sys))
	hist.idle.RecordValue(int64(delta.idle))
	hist.iowait.RecordValue(int64(delta.iowait))
	hist.irq.RecordValue(int64(delta.irq))
	hist.softirq.RecordValue(int64(delta.softirq))
	hist.steal.RecordValue(int64(delta.steal))
	hist.guest.RecordValue(int64(delta.guest))
	hist.guestNice.RecordValue(int64(delta.guestNice))
	hist.ctxt.RecordValue(int64(delta.ctxt))
	hist.procsTotal.RecordValue(int64(delta.procsTotal))
	hist.procsRunning.RecordValue(int64(cur.procsRunning))
	hist.procsBlocked.RecordValue(int64(cur.procsBlocked))

	return delta
}
