package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// from pidlist, find all tasks, read and summarize their stats
func schedReaderPids2(pidlist pidlist) schedStatsMap {
	ret := make(schedStatsMap)
	for _, pid := range pidlist {
		var taskDir *os.File
		var taskNames []string
		var err error

		if taskDir, err = os.Open(fmt.Sprintf("/proc/%d/task", pid)); err != nil {
			continue
		}
		if taskNames, err = taskDir.Readdirnames(0); err != nil {
			log.Fatal("Readdirnames: ", err)
		}
		var taskid int
		pidSum := schedStats{}
		fmt.Printf("pid=%d tasks=%v\n", pid, taskNames)
		for _, taskName := range taskNames {
			// skip non-numbers that might happen to be in this dir
			if taskid, err = strconv.Atoi(taskName); err != nil {
				continue
			}
			lines, err := readFileLines(fmt.Sprintf("/proc/%d/task/%d/sched", pid, taskid))
			// proc or thread could have exited between when we scanned the dir and now
			if err != nil {
				continue
			}
			for _, line := range lines[2:] {
				parts := strings.Split(line, ":")
				if len(parts) != 2 {
					continue
				}
				parts[0] = strings.TrimSpace(parts[0])
				parts[1] = strings.TrimSpace(parts[1])
				switch parts[0] {
				case "se.vruntime":
					pidSum.vruntime += readFloat(parts[1])
				case "se.sum_exec_runtime":
					pidSum.execRuntime += readFloat(parts[1])
				case "se.statistics.wait_sum":
					pidSum.waitSum += readFloat(parts[1])
					fmt.Println(pidSum.waitSum, parts[1])
				case "se.statistics.wait_count":
					pidSum.waitCount += readUInt(parts[1])
				case "se.statistics.iowait_sum":
					pidSum.iowaitSum += readFloat(parts[1])
				case "se.statistics.iowait_count":
					pidSum.iowaitCount += readUInt(parts[1])
				case "nr_switches":
					pidSum.nrSwitches += readUInt(parts[1])
				case "nr_voluntary_switches":
					pidSum.nrVoluntarySwitches += readUInt(parts[1])
				case "nr_involuntary_switches":
					pidSum.nrInvoluntarySwitches += readUInt(parts[1])
				case "clock-delta":
					pidSum.clockDelta = readUInt(parts[1])
				}
			}
		}
		ret[pid] = &pidSum
	}
	return ret
}
