package main

import (
	"log"
	"os"
	"strconv"
)

type pidlist []int

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
}
