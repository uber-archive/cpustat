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

package cpustat

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

var procPath = "/proc"

type Pidlist []int

// We churn the pidlist constantly, so this is an optimization to reuse the underlying list every time.
// Replace the new values in the old list, shrinking or growing as necessary. This saves a bit of GC.
// Note that reading /proc to get the pidlist returns the elements in a consistent order. If we ever
// get a new source of a pidlist like perf_events or something, make sure it sorts.
func GetPidList(list *Pidlist, maxProcsToScan int) {
	var procDir *os.File
	var procNames []string
	var err error

	if procDir, err = os.Open(procPath); err != nil {
		log.Fatalf("Open dir %s:%s", procPath, err)
	}
	if procNames, err = procDir.Readdirnames(maxProcsToScan); err != nil {
		log.Fatal("pidlist Readdirnames: ", err)
	}
	if len(procNames) > maxProcsToScan-1 {
		fmt.Println("proc table truncated because more than", maxProcsToScan, "procs found")
	}
	procDir.Close()

	*list = (*list)[:0]
	var pid int

	for _, fileName := range procNames {
		if pid, err = strconv.Atoi(fileName); err != nil {
			continue
		}
		*list = append(*list, pid)
	}
}
