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
	"io/ioutil"
	"os"
	"testing"
)

func tmpFile(contents string) (*os.File, error) {
	tmpfile, err := ioutil.TempFile("", "system_stats_test.go")
	if err != nil {
		return tmpfile, err
	}

	if _, err := tmpfile.Write([]byte(contents)); err != nil {
		return tmpfile, err
	}
	if err := tmpfile.Close(); err != nil {
		return tmpfile, err
	}
	StatsPath = tmpfile.Name()

	return tmpfile, nil
}

func TestEmptyFile(t *testing.T) {
	file, err := tmpFile("")
	defer os.Remove(file.Name())
	if err != nil {
		t.Error(err)
	}

	sys := SystemStats{}
	err = SystemStatsReader(&sys)
	if err == nil {
		t.Error("empty stats file should be an error but isn't")
	}
}

var pre2633 = `cpu  130 1 493 10614 387 20 13 2 3
cpu0 24 0 98 2698 108 20 1 0 0
cpu1 29 0 132 2658 96 0 3 0 0
cpu2 39 0 90 2692 92 0 5 0 0
cpu3 36 0 170 2565 90 0 4 0 0
intr 37310 121 7 0 0 0 0 0 0 0 0 0 0 112 0 0 69 161 0 0 29 0 2675 26 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
ctxt 23132
btime 1459206723
processes 1372
procs_running 1
procs_blocked 2
softirq 46994 0 18265 7 190 2718 0 2 3667 4 22141`

func TestLinuxPre2633(t *testing.T) {
	file, err := tmpFile(pre2633)
	defer os.Remove(file.Name())
	if err != nil {
		t.Error(err)
	}

	stats := SystemStats{}
	err = SystemStatsReader(&stats)
	if err != nil {
		t.Error(err)
	}
	if stats.Usr != 130 {
		t.Error("usr should be 130 but is", stats.Usr)
	}
	if stats.Nice != 1 {
		t.Error("nice should be 1 but is", stats.Nice)
	}
	if stats.Sys != 493 {
		t.Error("sys should be 493 but is", stats.Sys)
	}
	if stats.Idle != 10614 {
		t.Error("idle should be 10614 but is", stats.Idle)
	}
	if stats.Iowait != 387 {
		t.Error("iowait should be 387 but is", stats.Iowait)
	}
	if stats.Irq != 20 {
		t.Error("irq should be 20 but is", stats.Irq)
	}
	if stats.Softirq != 13 {
		t.Error("softirq should be 13 but is", stats.Softirq)
	}
	if stats.Steal != 2 {
		t.Error("steal should be 2 but is", stats.Steal)
	}
	if stats.Guest != 3 {
		t.Error("guest should be 3 but is", stats.Guest)
	}
	if stats.GuestNice != 0 {
		t.Error("guestNice should be 0 but is", stats.GuestNice)
	}
	if stats.Ctxt != 23132 {
		t.Error("ctxt should be 23132 but is", stats.Ctxt)
	}
	if stats.ProcsTotal != 1372 {
		t.Error("procsTotal should be 1372 but is", stats.ProcsTotal)
	}
	if stats.ProcsRunning != 1 {
		t.Error("procsRunning should be 1 but is", stats.ProcsRunning)
	}
	if stats.ProcsBlocked != 2 {
		t.Error("procsBlocked should be 2 but is", stats.ProcsBlocked)
	}
}

var post2633 = `cpu  10327 621 4341 21223299 2092 2 643 6 7 8
cpu0 3702 602 1258 5301320 910 2 373 0 0 0
cpu1 2104 18 957 5307634 240 0 91 0 0 0
cpu2 2499 0 1259 5306604 502 0 97 0 0 0
cpu3 2021 0 866 5307740 439 0 81 0 0 0
intr 1537790 33 12 0 0 0 0 0 0 0 0 0 0 156 0 0 51963 20912 0 0 2507 21297 63444 27 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
ctxt 2752498
btime 1459152751
processes 26946
procs_running 2
procs_blocked 3
softirq 742107 2 283236 90 22753 81771 0 47 242210 486 111512`

func TestLinuxPost2633(t *testing.T) {
	file, err := tmpFile(post2633)
	defer os.Remove(file.Name())
	if err != nil {
		t.Error(err)
	}

	stats := SystemStats{}
	err = SystemStatsReader(&stats)
	if err != nil {
		t.Error(err)
	}
	if stats.Usr != 10327 {
		t.Error("usr should be 10327 but is", stats.Usr)
	}
	if stats.Nice != 621 {
		t.Error("nice should be 621 but is", stats.Nice)
	}
	if stats.Sys != 4341 {
		t.Error("sys should be 4341 but is", stats.Sys)
	}
	if stats.Idle != 21223299 {
		t.Error("idle should be 21223299 but is", stats.Idle)
	}
	if stats.Iowait != 2092 {
		t.Error("iowait should be 2092 but is", stats.Iowait)
	}
	if stats.Irq != 2 {
		t.Error("irq should be 2 but is", stats.Irq)
	}
	if stats.Softirq != 643 {
		t.Error("softirq should be 643 but is", stats.Softirq)
	}
	if stats.Steal != 6 {
		t.Error("steal should be 6 but is", stats.Steal)
	}
	if stats.Guest != 7 {
		t.Error("guest should be 7 but is", stats.Guest)
	}
	if stats.GuestNice != 8 {
		t.Error("guestNice should be 8 but is", stats.GuestNice)
	}
	if stats.Ctxt != 2752498 {
		t.Error("ctxt should be 2752498 but is", stats.Ctxt)
	}
	if stats.ProcsTotal != 26946 {
		t.Error("procsTotal should be 26946 but is", stats.ProcsTotal)
	}
	if stats.ProcsRunning != 2 {
		t.Error("procsRunning should be 2 but is", stats.ProcsRunning)
	}
	if stats.ProcsBlocked != 3 {
		t.Error("procsBlocked should be 3 but is", stats.ProcsBlocked)
	}
}
