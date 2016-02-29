package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/codahale/hdrhistogram"
)

type taskStatsHist struct {
	ustime *hdrhistogram.Histogram
	iowait *hdrhistogram.Histogram
	swap   *hdrhistogram.Histogram
}

type taskStatsMap map[int]*taskStats
type taskStatsHistMap map[int]*taskStatsHist

func statReader(conn *NLConn, pids pidlist, cmdNames cmdlineMap) taskStatsMap {
	cur := make(taskStatsMap)

	for _, pid := range pids {
		stat, err := lookupPid(conn, pid)
		if err != nil {
			fmt.Println("skipping pid", pid, err)
			continue
		}
		if stat.comm == "cpustat" {
			fmt.Printf("stat: %+v\n", stat)
		}
		cur[pid] = stat
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

func statRecord(interval int, curMap, prevMap, sumMap taskStatsMap, histMap taskStatsHistMap) taskStatsMap {
	deltaMap := make(taskStatsMap)

	for pid, cur := range curMap {
		if prev, ok := prevMap[pid]; ok == true {
			if _, ok := sumMap[pid]; ok == false {
				sumMap[pid] = &taskStats{}
			}
			deltaMap[pid] = &taskStats{}
			delta := deltaMap[pid]

			delta.captureTime = cur.captureTime
			delta.prevTime = prev.captureTime
			duration := float64(delta.captureTime.Sub(delta.prevTime) / time.Millisecond)
			scale := float64(interval) / duration

			sum := sumMap[pid]
			sum.captureTime = cur.captureTime

			sum.version = cur.version
			sum.exitcode = cur.exitcode
			sum.flag = cur.flag
			sum.nice = cur.nice
			delta.cpudelaycount = scaledSub(cur.cpudelaycount, prev.cpudelaycount, scale)
			sum.cpudelaycount += safeSub(cur.cpudelaycount, prev.cpudelaycount)
			delta.cpudelaytotal = scaledSub(cur.cpudelaytotal, prev.cpudelaytotal, scale)
			sum.cpudelaytotal += safeSub(cur.cpudelaytotal, prev.cpudelaytotal)
			delta.blkiodelaycount = scaledSub(cur.blkiodelaycount, prev.blkiodelaycount, scale)
			sum.blkiodelaycount += safeSub(cur.blkiodelaycount, prev.blkiodelaycount)
			delta.blkiodelaytotal = scaledSub(cur.blkiodelaytotal, prev.blkiodelaytotal, scale)
			sum.blkiodelaytotal += safeSub(cur.blkiodelaytotal, prev.blkiodelaytotal)
			delta.swapindelaycount = scaledSub(cur.swapindelaycount, prev.swapindelaycount, scale)
			sum.swapindelaycount += safeSub(cur.swapindelaycount, prev.swapindelaycount)
			delta.swapindelaytotal = scaledSub(cur.swapindelaytotal, prev.swapindelaytotal, scale)
			sum.swapindelaytotal += safeSub(cur.swapindelaytotal, prev.swapindelaytotal)
			delta.cpurunrealtotal = scaledSub(cur.cpurunrealtotal, prev.cpurunrealtotal, scale)
			sum.cpurunrealtotal += safeSub(cur.cpurunrealtotal, prev.cpurunrealtotal)
			delta.cpurunvirtualtotal = scaledSub(cur.cpurunvirtualtotal, prev.cpurunvirtualtotal, scale)
			sum.cpurunvirtualtotal += safeSub(cur.cpurunvirtualtotal, prev.cpurunvirtualtotal)
			sum.comm = cur.comm
			sum.sched = cur.sched
			sum.uid = cur.uid
			sum.gid = cur.gid
			sum.pid = cur.pid
			sum.ppid = cur.ppid
			sum.btime = cur.btime
			delta.etime = scaledSub(cur.etime, prev.etime, scale)
			sum.etime += safeSub(cur.etime, prev.etime)
			delta.utime = scaledSub(cur.utime, prev.utime, scale)
			sum.utime += safeSub(cur.utime, prev.utime)
			delta.stime = scaledSub(cur.stime, prev.stime, scale)
			sum.stime += safeSub(cur.stime, prev.stime)
			delta.minflt = scaledSub(cur.minflt, prev.minflt, scale)
			sum.minflt += safeSub(cur.minflt, prev.minflt)
			delta.majflt = scaledSub(cur.majflt, prev.majflt, scale)
			sum.majflt += safeSub(cur.majflt, prev.majflt)
			delta.coremem = scaledSub(cur.coremem, prev.coremem, scale)
			sum.coremem += safeSub(cur.coremem, prev.coremem)
			delta.virtmem = scaledSub(cur.virtmem, prev.virtmem, scale)
			sum.virtmem += safeSub(cur.virtmem, prev.virtmem)
			sum.hiwaterrss = cur.hiwaterrss
			sum.hiwatervm = cur.hiwatervm
			delta.readchar = scaledSub(cur.readchar, prev.readchar, scale)
			sum.readchar += safeSub(cur.readchar, prev.readchar)
			delta.writechar = scaledSub(cur.writechar, prev.writechar, scale)
			sum.writechar += safeSub(cur.writechar, prev.writechar)
			delta.readsyscalls = scaledSub(cur.readsyscalls, prev.readsyscalls, scale)
			sum.readsyscalls += safeSub(cur.readsyscalls, prev.readsyscalls)
			delta.writesyscalls = scaledSub(cur.writesyscalls, prev.writesyscalls, scale)
			sum.writesyscalls += safeSub(cur.writesyscalls, prev.writesyscalls)
			delta.readbytes = scaledSub(cur.readbytes, prev.readbytes, scale)
			sum.readbytes += safeSub(cur.readbytes, prev.readbytes)
			delta.writebytes = scaledSub(cur.writebytes, prev.writebytes, scale)
			sum.writebytes += safeSub(cur.writebytes, prev.writebytes)
			delta.cancelledwritebytes = scaledSub(cur.cancelledwritebytes, prev.cancelledwritebytes, scale)
			sum.cancelledwritebytes += safeSub(cur.cancelledwritebytes, prev.cancelledwritebytes)
			delta.nvcsw = scaledSub(cur.nvcsw, prev.nvcsw, scale)
			sum.nvcsw += safeSub(cur.nvcsw, prev.nvcsw)
			delta.nivcsw = scaledSub(cur.nivcsw, prev.nivcsw, scale)
			sum.nivcsw += safeSub(cur.nivcsw, prev.nivcsw)
			delta.utimescaled = scaledSub(cur.utimescaled, prev.utimescaled, scale)
			sum.utimescaled += safeSub(cur.utimescaled, prev.utimescaled)
			delta.stimescaled = scaledSub(cur.stimescaled, prev.stimescaled, scale)
			sum.stimescaled += safeSub(cur.stimescaled, prev.stimescaled)
			delta.cpuscaledrunrealtotal = scaledSub(cur.cpuscaledrunrealtotal, prev.cpuscaledrunrealtotal, scale)
			sum.cpuscaledrunrealtotal += safeSub(cur.cpuscaledrunrealtotal, prev.cpuscaledrunrealtotal)
			delta.freepagescount = scaledSub(cur.freepagescount, prev.freepagescount, scale)
			sum.freepagescount += safeSub(cur.freepagescount, prev.freepagescount)
			delta.freepagesdelaytotal = scaledSub(cur.freepagesdelaytotal, prev.freepagesdelaytotal, scale)
			sum.freepagesdelaytotal += safeSub(cur.freepagesdelaytotal, prev.freepagesdelaytotal)

			var hist *taskStatsHist
			if hist, ok = histMap[pid]; ok != true {
				histMap[pid] = &taskStatsHist{
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
					hdrhistogram.New(histMin, histMax, histSigFigs),
				}
				hist = histMap[pid]
			}
			hist.ustime.RecordValue(int64(delta.utime + delta.stime))
			hist.iowait.RecordValue(int64(delta.blkiodelaytotal))
			hist.swap.RecordValue(int64(delta.swapindelaytotal))
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
