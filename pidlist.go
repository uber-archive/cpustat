package main

import (
	"log"
	"os"
	"strconv"
)

type pidlist []int

// we churn the pidlist constantly, so this is an optimization to reuse the underlying list every time
// replace the new values in the old list, shrinking or growing as necessary
func getPidList(list *pidlist) {
	var procDir *os.File
	var procNames []string
	var err error

	if procDir, err = os.Open("/proc"); err != nil {
		log.Fatal("Open dir /proc: ", err)
	}
	if procNames, err = procDir.Readdirnames(maxProcsToScan); err != nil {
		log.Fatal("pidlist Readdirnames: ", err)
	}

	var pid int
	i := 0
	for _, fileName := range procNames {
		if pid, err = strconv.Atoi(fileName); err != nil {
			continue
		}
		if i >= len(*list) {
			*list = append(*list, pid)
		} else {
			(*list)[i] = pid
		}
		i++
	}
	if len(*list) > i {
		*list = (*list)[:i]
	}
}
