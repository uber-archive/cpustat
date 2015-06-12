package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

var logger *log.Logger

type times struct {
	usr  float64
	sys  float64
	cusr float64
	csys float64
}

func main() {
	interval := flag.Int("i", 1000, "interval between updates")
	pids := flag.String("pids", "", "process ids to monitor")
	flag.Parse()

	if *pids == "" {
		flag.Usage()
		os.Exit(1)
	}

	logger = log.New(os.Stdout, "", log.Ltime|log.Lmicroseconds)

	var iSec = float64(*interval) / 1000
	var pidList []int
	pidStripped := strings.Replace(*pids, "\n", " ", -1)
	pidStripped = strings.Replace(pidStripped, "\r", "", -1)
	for _, pidStr := range strings.Split(pidStripped, " ") {
		newPid, err := strconv.ParseUint(pidStr, 10, 32)
		if err == nil {
			pidList = append(pidList, int(newPid))
		}
	}

	fmt.Println("Pids: ", pidList)
	prev := make(map[int]*times)
	for {
		for _, pid := range pidList {
			file, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
			if err != nil {
				logger.Fatal(err)
			}
			fileStr := string(file)
			parts := strings.Split(fileStr, " ")

			usr := readFloat(parts[13])
			sys := readFloat(parts[14])
			cusr := readFloat(parts[15])
			csys := readFloat(parts[16])

			last, ok := prev[pid]
			if ok == true {
				usrPct := (((usr - last.usr) / 100) / iSec) * 100
				sysPct := (((sys - last.sys) / 100) / iSec) * 100
				cusrPct := (((cusr - last.cusr) / 100) / iSec) * 100
				csysPct := (((csys - last.csys) / 100) / iSec) * 100
				logger.Printf("%7d %3.0f/%3.0f child: %3.0f/%3.0f", pid, usrPct, sysPct, cusrPct, csysPct)
			}
			prev[pid] = &times{usr, sys, cusr, csys}
		}
		time.Sleep(time.Duration(*interval) * time.Millisecond)
	}
}

func readFloat(str string) float64 {
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		logger.Fatal(err)
	}
	return val
}
