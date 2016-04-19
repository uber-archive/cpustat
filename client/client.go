package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"

	"github.com/uber-common/cpustat/lib"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/raw"
)

func main() {
	var fetchCount = flag.Int("c", 300, "number of samples to summarize")
	var hostPort = flag.String("host", "127.0.0.1:1971", "hostport to fetch samples from")
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	var memprofile = flag.String("memprofile", "", "write memory profile to this file")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		if err = pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		<-sigChan

		if *cpuprofile != "" {
			pprof.StopCPUProfile()
		}

		if *memprofile != "" {
			f, err := os.Create(*memprofile)
			if err != nil {
				log.Fatal(err)
			}
			pprof.WriteHeapProfile(f)
			f.Close()
		}

		os.Exit(0)
	}()

	ch, err := tchannel.NewChannel("cpustat", nil)
	if err != nil {
		log.Fatalf("NewChannel failed: %v", err)
	}

	ctx, cancel := tchannel.NewContext(5 * time.Second)
	defer cancel()

	sendCount := make([]byte, 4)
	binary.LittleEndian.PutUint32(sendCount, uint32(*fetchCount))

	_, arg3, _, err := raw.Call(ctx, ch, *hostPort, "cpustat", "readSamples", nil, sendCount)
	if err != nil {
		panic(err)
	}

	buf := bytes.NewBuffer(arg3)
	dec := gob.NewDecoder(buf)
	var when time.Time
	err = dec.Decode(&when)
	var infoMap cpustat.ProcInfoMap
	err = dec.Decode(&infoMap)
	var interval uint32
	err = dec.Decode(&interval)
	var recvCount uint32
	err = dec.Decode(&recvCount)
	procSum := newProcSum(interval, infoMap)
	for i := uint32(0); i < recvCount; i++ {
		var procList []cpustat.ProcSample
		var sys cpustat.SystemStats

		err = dec.Decode(&procList)
		err = dec.Decode(&sys)

		procSum.update(procList, &sys)
	}

	procSum.summarize()

	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}
}

type procSummary struct {
	infoMap cpustat.ProcInfoMap

	procCur   []cpustat.ProcSample
	procPrev  []cpustat.ProcSample
	procSum   cpustat.ProcSampleMap
	procDelta cpustat.ProcSampleMap

	procHist cpustat.ProcStatsHistMap
	taskHist cpustat.TaskStatsHistMap

	sysCur   *cpustat.SystemStats
	sysPrev  *cpustat.SystemStats
	sysSum   *cpustat.SystemStats
	sysDelta *cpustat.SystemStats

	sysHist *cpustat.SystemStatsHist

	Interval uint32
	Samples  uint32
}

func newProcSum(interval uint32, infoMap cpustat.ProcInfoMap) *procSummary {
	ret := procSummary{}

	ret.infoMap = infoMap

	ret.procSum = make(cpustat.ProcSampleMap)
	ret.procDelta = make(cpustat.ProcSampleMap)

	ret.procHist = make(cpustat.ProcStatsHistMap)
	ret.taskHist = make(cpustat.TaskStatsHistMap)

	ret.sysCur = &cpustat.SystemStats{}
	ret.sysPrev = &cpustat.SystemStats{}
	ret.sysSum = &cpustat.SystemStats{}
	ret.sysDelta = &cpustat.SystemStats{}

	ret.sysHist = cpustat.NewSysStatsHist()

	ret.Interval = interval

	return &ret
}

func (p *procSummary) update(procSamples []cpustat.ProcSample, sys *cpustat.SystemStats) {
	if p.Samples == 0 {
		p.procPrev = procSamples
		p.sysPrev = sys
	} else {
		p.procCur = procSamples
		cur := cpustat.ProcSampleList{p.procCur, uint32(len(p.procCur))}
		prev := cpustat.ProcSampleList{p.procPrev, uint32(len(p.procPrev))}
		cpustat.ProcStatsRecord(p.Interval, cur, prev, p.procSum, p.procDelta)
		cpustat.TaskStatsRecord(p.Interval, cur, prev, p.procSum, p.procDelta)
		cpustat.UpdateProcStatsHist(p.procHist, p.procDelta)
		cpustat.UpdateTaskStatsHist(p.taskHist, p.procDelta)
		p.procPrev = p.procCur

		p.sysCur = sys
		p.sysDelta = cpustat.SystemStatsRecord(p.Interval, p.sysCur, p.sysPrev, p.sysSum)
		cpustat.UpdateSysStatsHist(p.sysHist, p.sysDelta)
		p.sysPrev = p.sysCur
	}

	p.Samples++
}

type sysJSON struct {
	Samples uint64

	UsrMin float64
	UsrMax float64
	UsrAvg float64
	UsrP95 float64

	NiceMin float64
	NiceMax float64
	NiceAvg float64
	NiceP95 float64

	SysMin float64
	SysMax float64
	SysAvg float64
	SysP95 float64

	IdleMin float64
	IdleMax float64
	IdleAvg float64
	IdleP95 float64

	IowaitMin float64
	IowaitMax float64
	IowaitAvg float64
	IowaitP95 float64

	ProcsTotalMin float64
	ProcsTotalMax float64
	ProcsTotalAvg float64
	ProcsTotalP95 float64

	ProcsRunningMin float64
	ProcsRunningMax float64
	ProcsRunningAvg float64
	ProcsRunningP95 float64

	ProcsBlockedMin float64
	ProcsBlockedMax float64
	ProcsBlockedAvg float64
	ProcsBlockedP95 float64
}

type procJSONEntry struct {
	Samples    uint64
	Pid        uint64
	Ppid       uint64
	Nice       int64
	Comm       string
	Cmdline    []string
	Numthreads uint64

	RSS uint64

	UsrMin float64
	UsrMax float64
	UsrAvg float64
	UsrP95 float64

	SysMin float64
	SysMax float64
	SysAvg float64
	SysP95 float64

	CUSMin float64
	CUSMax float64
	CUSAvg float64
	CUSP95 float64

	RunQAvg   float64
	RunQCount uint64
	IOWAvg    float64
	IOWCount  uint64
	SwapAvg   float64
	SwapCount uint64
	Vcsw      uint64
	Ivcsw     uint64
}

type sumJSON struct {
	Sys  sysJSON
	Proc []procJSONEntry
}

func (p *procSummary) summarize() {
	out := sumJSON{}

	out.Sys.Samples = uint64(p.sysHist.Usr.TotalCount())

	out.Sys.UsrMin = float64(p.sysHist.Usr.Min())
	out.Sys.UsrMax = float64(p.sysHist.Usr.Max())
	out.Sys.UsrAvg = p.sysHist.Usr.Mean()
	out.Sys.UsrP95 = float64(p.sysHist.Usr.ValueAtQuantile(95))

	out.Sys.NiceMin = float64(p.sysHist.Nice.Min())
	out.Sys.NiceMax = float64(p.sysHist.Nice.Max())
	out.Sys.NiceAvg = p.sysHist.Nice.Mean()
	out.Sys.NiceP95 = float64(p.sysHist.Nice.ValueAtQuantile(95))

	out.Sys.SysMin = float64(p.sysHist.Sys.Min())
	out.Sys.SysMax = float64(p.sysHist.Sys.Max())
	out.Sys.SysAvg = p.sysHist.Sys.Mean()
	out.Sys.SysP95 = float64(p.sysHist.Sys.ValueAtQuantile(95))

	out.Sys.IdleMin = float64(p.sysHist.Idle.Min())
	out.Sys.IdleMax = float64(p.sysHist.Idle.Max())
	out.Sys.IdleAvg = p.sysHist.Idle.Mean()
	out.Sys.IdleP95 = float64(p.sysHist.Idle.ValueAtQuantile(95))

	out.Sys.IowaitMin = float64(p.sysHist.Iowait.Min())
	out.Sys.IowaitMax = float64(p.sysHist.Iowait.Max())
	out.Sys.IowaitAvg = p.sysHist.Iowait.Mean()
	out.Sys.IowaitP95 = float64(p.sysHist.Iowait.ValueAtQuantile(95))

	out.Sys.ProcsTotalMin = float64(p.sysHist.ProcsTotal.Min())
	out.Sys.ProcsTotalMax = float64(p.sysHist.ProcsTotal.Max())
	out.Sys.ProcsTotalAvg = p.sysHist.ProcsTotal.Mean()
	out.Sys.ProcsTotalP95 = float64(p.sysHist.ProcsTotal.ValueAtQuantile(95))

	out.Sys.ProcsRunningMin = float64(p.sysHist.ProcsRunning.Min())
	out.Sys.ProcsRunningMax = float64(p.sysHist.ProcsRunning.Max())
	out.Sys.ProcsRunningAvg = p.sysHist.ProcsRunning.Mean()
	out.Sys.ProcsRunningP95 = float64(p.sysHist.ProcsRunning.ValueAtQuantile(95))

	out.Sys.ProcsBlockedMin = float64(p.sysHist.ProcsBlocked.Min())
	out.Sys.ProcsBlockedMax = float64(p.sysHist.ProcsBlocked.Max())
	out.Sys.ProcsBlockedAvg = p.sysHist.ProcsBlocked.Mean()
	out.Sys.ProcsBlockedP95 = float64(p.sysHist.ProcsBlocked.ValueAtQuantile(95))

	out.Proc = make([]procJSONEntry, 0, len(p.procSum))

	for pid, sum := range p.procSum {
		info, ok := p.infoMap[pid]
		if ok == false {
			panic(fmt.Sprint("missing procInfo for", pid))
		}
		hist, ok := p.procHist[pid]
		if ok == false {
			panic(fmt.Sprint("missing procHist for", pid))
		}

		entry := procJSONEntry{}
		entry.Pid = info.Pid
		entry.Comm = info.Comm
		entry.Ppid = info.Ppid
		entry.Nice = info.Nice
		entry.Cmdline = info.Cmdline

		entry.RSS = sum.Proc.Rss
		entry.Numthreads = sum.Proc.Numthreads

		entry.Samples = uint64(hist.Ustime.TotalCount())

		entry.UsrMin = float64(hist.Utime.Min())
		entry.UsrMax = float64(hist.Utime.Max())
		entry.UsrAvg = hist.Utime.Mean()
		entry.UsrP95 = float64(hist.Utime.ValueAtQuantile(95))

		entry.SysMin = float64(hist.Stime.Min())
		entry.SysMax = float64(hist.Stime.Max())
		entry.SysAvg = hist.Stime.Mean()
		entry.SysP95 = float64(hist.Stime.ValueAtQuantile(95))

		entry.CUSMin = float64(hist.Ustime.Min())
		entry.CUSMax = float64(hist.Ustime.Max())
		entry.CUSAvg = hist.Ustime.Mean()
		entry.CUSP95 = float64(hist.Ustime.ValueAtQuantile(95))

		entry.RunQAvg = float64(sum.Task.Cpudelaytotal)
		entry.RunQCount = sum.Task.Cpudelaycount
		entry.IOWAvg = float64(sum.Task.Blkiodelaytotal)
		entry.IOWCount = sum.Task.Blkiodelaycount
		entry.SwapAvg = float64(sum.Task.Swapindelaytotal)
		entry.SwapCount = sum.Task.Swapindelaycount

		out.Proc = append(out.Proc, entry)
	}

	b, err := json.Marshal(out)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}

func summarizeSys(allSamples []cpustat.SystemStats) {
	sum := cpustat.SystemStats{}
	hist := cpustat.NewSysStatsHist()
	for pos := len(allSamples) - 1; pos >= 1; pos-- {
		delta := cpustat.SystemStatsRecord(200, &allSamples[pos-1], &allSamples[pos], &sum)
		cpustat.UpdateSysStatsHist(hist, delta)
	}
	fmt.Println("Idle: ", hist.Idle.Min(), hist.Idle.Max(), hist.Idle.Mean(),
		hist.Idle.ValueAtQuantile(50), hist.Idle.ValueAtQuantile(90), hist.Idle.ValueAtQuantile(99))
	fmt.Println("Usr: ", hist.Usr.Min(), hist.Usr.Max(), hist.Usr.Mean(),
		hist.Usr.ValueAtQuantile(50), hist.Usr.ValueAtQuantile(90), hist.Usr.ValueAtQuantile(99))
	fmt.Println("Sys: ", hist.Sys.Min(), hist.Sys.Max(), hist.Sys.Mean(),
		hist.Sys.ValueAtQuantile(50), hist.Sys.ValueAtQuantile(90), hist.Sys.ValueAtQuantile(99))
	fmt.Println("Nice: ", hist.Nice.Min(), hist.Nice.Max(), hist.Nice.Mean(),
		hist.Nice.ValueAtQuantile(50), hist.Nice.ValueAtQuantile(90), hist.Nice.ValueAtQuantile(99))
	fmt.Println("IOWait: ", hist.Iowait.Min(), hist.Iowait.Max(), hist.Iowait.Mean(),
		hist.Iowait.ValueAtQuantile(50), hist.Iowait.ValueAtQuantile(90), hist.Iowait.ValueAtQuantile(99))
}

func fetchSamples(hostPort string, count int) {
	ch, err := tchannel.NewChannel("cpustat", nil)
	if err != nil {
		log.Fatalf("NewChannel failed: %v", err)
	}

	ctx, cancel := tchannel.NewContext(100 * time.Millisecond)
	defer cancel()

	sendCount := make([]byte, 4)
	binary.LittleEndian.PutUint32(sendCount, 10)
	_, arg3, _, err := raw.Call(ctx, ch, hostPort, "cpustat", "readSamples", nil, sendCount)
	if err != nil {
		panic(err)
	}

	buf := bytes.NewBuffer(arg3)
	dec := gob.NewDecoder(buf)
	var when time.Time
	err = dec.Decode(&when)
	var infoMap cpustat.ProcInfoMap
	err = dec.Decode(&infoMap)
	fmt.Println("pidlist", infoMap, err)
	var recvCount uint32
	err = dec.Decode(&recvCount)
	for i := uint32(0); i < recvCount; i++ {
		var procs cpustat.ProcStatsMap
		var tasks cpustat.TaskStatsMap
		var sys cpustat.SystemStats

		err = dec.Decode(&procs)
		err = dec.Decode(&tasks)
		err = dec.Decode(&sys)
	}
}
