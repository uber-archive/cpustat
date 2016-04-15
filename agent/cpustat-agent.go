// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// This program gathers the metrics from the system and writes them to lmdb where
// various other programs can come and get them

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/uber-common/cpustat/lib"
)

var infoMap cpustat.ProcInfoMap
var infolock sync.Mutex
var intervalms uint32

func main() {
	runtime.MemProfileRate = 1
	var interval = flag.Int("i", 200, "interval (ms) between measurements")
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	var memprofile = flag.String("memprofile", "", "write memory profile to this file")
	var dbSize = flag.Int("dbsize", 3000, "samples to keep in memory")
	var maxProcsToScan = flag.Int("maxprocs", 3000, "max size of process table")
	var statsInterval = flag.String("statsinterval", "1s", "print usage statistics to stdout, 0s to disable")
	//	var pruneChance = flag.Float64("prunechance", 0.01, "percentage of intervals to also prune old cmdline data")

	if os.Geteuid() != 0 {
		fmt.Println("This program uses the netlink taskstats inteface, so it must be run as root.")
		os.Exit(1)
	}

	flag.Parse()

	if *interval <= 10 {
		fmt.Println("The minimum sampling interval is 10ms")
		os.Exit(1)
	}
	intervalms = uint32(*interval)

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

	dbInit(uint32(*dbSize))
	//	expiry := time.Duration(*dbSize**interval) * time.Millisecond

	//	nlConn := cpustat.NLInit()

	sys := &cpustat.SystemStats{}

	rand.Seed(time.Now().UnixNano())

	var t1, t2 time.Time

	pids := make(cpustat.Pidlist, 0, *maxProcsToScan)
	infoMap := make(cpustat.ProcInfoMap, *maxProcsToScan)
	cur := make(cpustat.ProcSampleList, 0, *maxProcsToScan)

	t1 = time.Now()
	cpustat.GetPidList(&pids, *maxProcsToScan)
	cpustat.ProcStatsReader(pids, &cur, infoMap)
	//	cpustat.TaskStatsReader(nlConn, pids, cur)
	cpustat.SystemStatsReader(sys)
	writeSample(cur, sys)

	go runServer()

	go printStats(*statsInterval)

	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	t2 = time.Now()

	targetSleep := time.Duration(*interval) * time.Millisecond
	adjustedSleep := adjustSleep(targetSleep, t1, t2)

	for {
		time.Sleep(adjustedSleep)

		// the time it takes to do all of the work will vary, so measure it each time and sleep for remainder
		t1 = time.Now()

		cur = make(cpustat.ProcSampleList, 0, *maxProcsToScan)

		//cpustat.GetPidList(&pids, *maxProcsToScan)
		//		infolock.Lock()
		// cpustat.ProcStatsReader(pids, &cur, infoMap)
		//		cpustat.TaskStatsReader(nlConn, pids, cur)
		//		infoMap.MaybePrune(*pruneChance, pids, expiry)
		//		infolock.Unlock()
		// _ = cpustat.SystemStatsReader(sys)
		writeSample(cur, sys)
		runtime.GC()
		t2 = time.Now()
		adjustedSleep = adjustSleep(targetSleep, t1, t2)
	}
}

func adjustSleep(target time.Duration, t1, t2 time.Time) time.Duration {
	adjustedSleep := target - t2.Sub(t1)

	// If we can't keep up, try to buy ourselves a little headroom by sleeping for a magic number of extra ms
	if adjustedSleep <= 0 {
		fmt.Fprintf(os.Stderr, "warning: work cycle took longer than sampling interval by %dms\n", adjustedSleep)
		adjustedSleep = target + (time.Duration(100) * time.Millisecond)
	}
	return adjustedSleep
}

func printStats(s string) {
	dur, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}

	for {
		var curUsage syscall.Rusage
		err = syscall.Getrusage(syscall.RUSAGE_SELF, &curUsage)
		if err != nil {
			panic(err)
		}
		pcount, scount := dbStats()
		fmt.Printf("rss: %.2fMB db entries: %d procs: %d sys: %d\n",
			float64(curUsage.Maxrss)/1024, dbCount(), pcount, scount)
		time.Sleep(dur)
	}
}
