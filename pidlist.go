package main

import (
	"log"
	"os"
	"strconv"
)

type pidlist []int

func getPidList() pidlist {
	var procDir *os.File
	var procNames []string
	var err error

	if procDir, err = os.Open("/proc"); err != nil {
		log.Fatal("Open dir /proc: ", err)
	}
	if procNames, err = procDir.Readdirnames(0); err != nil {
		log.Fatal("pidlist Readdirnames: ", err)
	}

	var pid int
	list := make(pidlist, len(procNames))
	i := 0
	for _, fileName := range procNames {
		if pid, err = strconv.Atoi(fileName); err != nil {
			continue
		}
		list[i] = pid
		i++
	}

	return list[:i]
}
