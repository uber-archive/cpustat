package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
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
		var procMap cpustat.ProcStatsMap
		var taskMap cpustat.TaskStatsMap
		var sys cpustat.SystemStats

		err = dec.Decode(&procMap)
		err = dec.Decode(&taskMap)
		err = dec.Decode(&sys)

		procSum.update(procMap, taskMap, &sys)
	}

	procSum.summarize()
}

type procSummary struct {
	infoMap   cpustat.ProcInfoMap
	procCur   cpustat.ProcStatsMap
	procPrev  cpustat.ProcStatsMap
	procSum   cpustat.ProcStatsMap
	procDelta cpustat.ProcStatsMap
	procHist  cpustat.ProcStatsHistMap
	taskCur   cpustat.TaskStatsMap
	taskPrev  cpustat.TaskStatsMap
	taskSum   cpustat.TaskStatsMap
	taskDelta cpustat.TaskStatsMap
	taskHist  cpustat.TaskStatsHistMap
	sysCur    *cpustat.SystemStats
	sysPrev   *cpustat.SystemStats
	sysSum    *cpustat.SystemStats
	sysDelta  *cpustat.SystemStats
	sysHist   *cpustat.SystemStatsHist
	Interval  uint32
	Samples   uint32
}

func newProcSum(interval uint32, infoMap cpustat.ProcInfoMap) *procSummary {
	ret := procSummary{}

	ret.infoMap = infoMap

	ret.procCur = make(cpustat.ProcStatsMap)
	ret.procPrev = make(cpustat.ProcStatsMap)
	ret.procSum = make(cpustat.ProcStatsMap)
	ret.procDelta = make(cpustat.ProcStatsMap)
	ret.procHist = make(cpustat.ProcStatsHistMap)

	ret.taskCur = make(cpustat.TaskStatsMap)
	ret.taskPrev = make(cpustat.TaskStatsMap)
	ret.taskSum = make(cpustat.TaskStatsMap)
	ret.taskDelta = make(cpustat.TaskStatsMap)
	ret.taskHist = make(cpustat.TaskStatsHistMap)

	ret.sysCur = &cpustat.SystemStats{}
	ret.sysPrev = &cpustat.SystemStats{}
	ret.sysSum = &cpustat.SystemStats{}
	ret.sysDelta = &cpustat.SystemStats{}
	ret.sysHist = cpustat.NewSysStatsHist()

	ret.Interval = interval

	return &ret
}

func (p *procSummary) update(procMap cpustat.ProcStatsMap, taskMap cpustat.TaskStatsMap, sys *cpustat.SystemStats) {
	if p.Samples == 0 {
		p.procPrev = procMap
		p.taskPrev = taskMap
		p.sysPrev = sys
	} else {
		p.procCur = procMap
		p.procDelta = cpustat.ProcStatsRecord(p.Interval, p.procCur, p.procPrev, p.procSum)
		for pid, delta := range p.procDelta {
			fmt.Printf("%d: %+v\n", pid, delta)
		}

		cpustat.UpdateProcStatsHist(p.procHist, p.procDelta)
		p.procPrev = p.procCur

		p.taskCur = taskMap
		p.taskDelta = cpustat.TaskStatsRecord(p.Interval, p.taskCur, p.taskPrev, p.taskSum)
		cpustat.UpdateTaskStatsHist(p.taskHist, p.taskDelta)
		p.taskPrev = p.taskCur

		p.sysCur = sys
		p.sysDelta = cpustat.SystemStatsRecord(p.Interval, p.sysCur, p.sysPrev, p.sysSum)
		cpustat.UpdateSysStatsHist(p.sysHist, p.sysDelta)
		p.sysPrev = p.sysCur
	}

	p.Samples++
}

type procJSON struct {
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

func (p *procSummary) summarize() {
	for pid, sum := range p.procSum {
		info, ok := p.infoMap[pid]
		if ok == false {
			panic(fmt.Sprint("missing procInfo for", pid))
		}
		hist, ok := p.procHist[pid]
		if ok == false {
			panic(fmt.Sprint("missing procHist for", pid))
		}

		out := procJSON{}
		out.Pid = info.Pid
		out.Comm = info.Comm
		out.Ppid = info.Ppid
		out.Nice = info.Nice
		out.Cmdline = info.Cmdline

		out.RSS = sum.Rss
		out.Numthreads = sum.Numthreads

		out.Samples = uint64(hist.Ustime.TotalCount())

		out.UsrMin = float64(hist.Utime.Min())
		out.UsrMax = float64(hist.Utime.Max())
		out.UsrAvg = hist.Utime.Mean()
		out.UsrP95 = float64(hist.Utime.ValueAtQuantile(95))

		out.SysMin = float64(hist.Stime.Min())
		out.SysMax = float64(hist.Stime.Max())
		out.SysAvg = hist.Stime.Mean()
		out.SysP95 = float64(hist.Stime.ValueAtQuantile(95))

		out.CUSMin = float64(hist.Ustime.Min())
		out.CUSMax = float64(hist.Ustime.Max())
		out.CUSAvg = hist.Ustime.Mean()
		out.CUSP95 = float64(hist.Ustime.ValueAtQuantile(95))

		if task, ok := p.taskSum[pid]; ok == true {
			out.RunQAvg = float64(task.Cpudelaytotal)
			out.RunQCount = task.Cpudelaycount
			out.IOWAvg = float64(task.Blkiodelaytotal)
			out.IOWCount = task.Blkiodelaycount
			out.SwapAvg = float64(task.Swapindelaytotal)
			out.SwapCount = task.Swapindelaycount
		} else {
			fmt.Println("no taskSum for", pid)
		}
		//		b, err := json.Marshal(out)
		//		if err != nil {
		//			panic(err)
		//		}
		//		fmt.Println(string(b))
	}
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
