package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/uber-common/cpustat/lib"
)

type pidList []int

func (pl *pidList) String() string {
	if pl == nil {
		return ""
	}
	ss := make([]string, len(*pl))
	for i, pid := range *pl {
		ss[i] = strconv.Itoa(pid)
	}
	return strings.Join(ss, ",")
}

func (pl *pidList) Set(s string) error {
	parts := strings.Split(s, ",")
	for _, part := range parts {
		pid, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("invalid pid list component %q, expected an integer", part)
		}
		*pl = append(*pl, pid)
	}
	return nil
}

func main() {
	var (
		interval    = flag.Duration("i", 200*time.Millisecond, "duration between measurements")
		sampleCount = flag.Uint("n", 0, "Maximum number of samples to capture")
		pids        pidList
	)
	flag.Var(&pids, "p", "Comma separated PID list to profile")
	flag.Parse()

	if len(pids) == 0 {
		fmt.Fprintf(os.Stderr, "no pids provided")
		os.Exit(1)
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
			if adjustedSleep := *interval - time.Now().Sub(startOfRead); adjustedSleep > 0 {
				// TODO actually probably some minimum sleep tolerance
				time.Sleep(adjustedSleep)
			}
		}
	}
}
