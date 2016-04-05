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

	ctx, cancel := tchannel.NewContext(100 * time.Millisecond)
	defer cancel()

	sendCount := make([]byte, 4)
	binary.LittleEndian.PutUint32(sendCount, uint32(*fetchCount))

	_, arg3, _, err := raw.Call(ctx, ch, *hostPort, "cpustat", "readSys", nil, sendCount)
	if err != nil {
		panic(err)
	}

	buf := bytes.NewBuffer(arg3)
	dec := gob.NewDecoder(buf)
	var when time.Time
	err = dec.Decode(&when)
	var recvCount uint32
	err = dec.Decode(&recvCount)
	fmt.Printf("Recv time: %v\n", when)
	fmt.Println("need to decode", recvCount, "samples", err)
	list := make([]cpustat.SystemStats, recvCount)
	for i := uint32(0); i < recvCount; i++ {
		var sys cpustat.SystemStats
		err = dec.Decode(&sys)
		list[i] = sys
	}
	summarize(list)
}

func summarize(allSamples []cpustat.SystemStats) {
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
	var cmdNames cpustat.CmdlineMap
	err = dec.Decode(&cmdNames)
	fmt.Println("pidlist", cmdNames, err)
	var recvCount uint32
	err = dec.Decode(&recvCount)
	fmt.Printf("Recv time: %v\n", when)
	fmt.Println("need to decode", recvCount, "samples", err)
	for i := uint32(0); i < recvCount; i++ {
		var procs cpustat.ProcStatsMap
		var tasks cpustat.TaskStatsMap
		var sys cpustat.SystemStats

		err = dec.Decode(&procs)
		err = dec.Decode(&tasks)
		err = dec.Decode(&sys)
	}
}
