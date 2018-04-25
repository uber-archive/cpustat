package main

import (
	"flag"
	"fmt"
	"github.com/uber-common/cpustat/lib"
	"strconv"
	"strings"
	"time"
)

func main() {
	var interval = flag.Int("i", 200, "Interval (ms) between measurements")
	var pidList = flag.String("p", "", "Comma separated PID list to profile")
	var sampleCount = flag.Uint("n", 0, "Maximum number of samples to capture")

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

	t1 := time.Now()

	for procStatsReaderCount > 0 && samplesRemaining != 0 {
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
			t2 := time.Now()
			adjustedSleep := targetSleep - t2.Sub(t1)
			time.Sleep(adjustedSleep)
			t1 = time.Now()
		}
	}
}
