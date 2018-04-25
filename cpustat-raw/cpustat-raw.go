package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/uber-common/cpustat/lib"
)

func main() {
	var (
		interval    = flag.Int("i", 200, "Interval (ms) between measurements")
		pidList     = flag.String("p", "", "Comma separated PID list to profile")
		sampleCount = flag.Uint("n", 0, "Maximum number of samples to capture")
	)

	flag.Parse()

	targetSleep := time.Duration(*interval) * time.Millisecond

	pidStrings := strings.Split(*pidList, ",")
	pids := make([]int, len(pidStrings))

	for i, pidString := range pidStrings {
		pid, err := strconv.Atoi(pidString)

		if err != nil {
			panic(err)
		}

		pids[i] = pid
	}

	procStats := cpustat.ProcStats{}
	procStatsReaderCount := len(pids)
	procStatsReaders := make([]*cpustat.ProcStatsSeekReader, procStatsReaderCount)

	samplesRemaining := int64(*sampleCount)
	if samplesRemaining == 0 {
		samplesRemaining = -1
	}

	for i, pid := range pids {
		procStatsReader := cpustat.ProcStatsSeekReader{
			PID: pid,
		}
		procStatsReaders[i] = &procStatsReader

		procStatsInitError := procStatsReader.Initialize()
		if procStatsInitError != nil {
			procStatsReaders[i] = nil
			procStatsReaderCount--
		}
	}

	fmt.Printf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
		"pid",
		"time",
		"proc.utime",
		"proc.stime",
		"proc.cutime",
		"proc.cstime",
		"proc.numthreads",
		"proc.rss",
		"proc.guesttime",
		"proc.cguesttime",
	)

	for procStatsReaderCount > 0 && samplesRemaining != 0 {
		startOfRead := time.Now()

		for i, procStatsReader := range procStatsReaders {
			if procStatsReader == nil {
				continue
			}

			procStatsError := procStatsReader.ReadStats(&procStats)
			if procStatsError != nil {
				procStatsReaders[i] = nil
				procStatsReaderCount--
				continue
			}

			fmt.Printf(
				"%d,%d,%d,%d,%d,%d,%d,%d,%d,%d\n",
				procStatsReader.PID,
				procStats.CaptureTime.UnixNano()/1e6,
				procStats.Utime,
				procStats.Stime,
				procStats.Cutime,
				procStats.Cstime,
				procStats.Numthreads,
				procStats.Rss,
				procStats.Guesttime,
				procStats.Cguesttime,
			)
		}

		if samplesRemaining > 0 {
			samplesRemaining--
		}

		if procStatsReaderCount > 0 && samplesRemaining != 0 {
			adjustedSleep := targetSleep - time.Now().Sub(startOfRead)
			time.Sleep(adjustedSleep)
		}
	}
}
