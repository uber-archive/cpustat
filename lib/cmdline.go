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

// This package reads and caches the results from /proc/pid/cmdline.
// It also perhaps surprisingly transforms these names into names that are more useful in
// some environments. It would be nice if there was some way to let people extend these
// rules based on how they run their programs.

// TODO handle these:
// udocker   85999 13.3  0.3 1742504 417780 ?      Sl   Apr06 933:29 python -m geosnapper.app /var/run/udocker/geosnapper-0.sock 0
// uber      52832  0.3  0.0 370500 35404 ?        Sl   Apr06  25:34 /usr/bin/python /usr/bin/sortsol_sender docker_daemon-access
// udocker   46233 53.6  0.0 260308 81976 ?        Rs   Apr08 2190:05 /usr/bin/python /usr/local/bin/celery worker --app=polaris -l INFO -Q polaris -c 5 --logfile=/var/log/udocker/polaris/polaris-celery.log
// udocker   43061  2.6  0.1 3338048 149152 ?      Sl   Apr09 102:54 /usr/bin/python /usr/local/bin/mastermind-tornado /var/run/udocker/mastermind-3.sock 3
// nsca      88499  0.0  0.0  40040 12908 ?        S    17:15   0:00 python /usr/local/sbin/nagios_autocheck_submitter --nagios-host=nagioscheck.local.uber.internal --splay 600

package cpustat

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"syscall"
	"time"
)

// ProcInfo holds properties of a process that don't change very often or can't change
type ProcInfo struct {
	FirstSeen  time.Time
	LastSeen   time.Time
	Comm       string   // short name from /proc/pid/stat
	Cmdline    []string // raw parts from /proc/pid/cmdline
	Friendly   string   // our magically transformed name
	Pid        uint64
	Ppid       uint64
	Pgrp       int64
	Session    int64
	Ttynr      int64
	Tpgid      int64
	Flags      uint64
	Starttime  uint64
	Nice       int64
	Rtpriority uint64
	Policy     uint64
	UID        uint32
}

type ProcInfoMap map[int]*ProcInfo

func (m ProcInfoMap) MaybePrune(chance float64, pids Pidlist, expiry time.Duration) {
	if rand.Float64() >= chance {
		return
	}

	pidMap := make(map[int]bool)
	for pid, _ := range pids {
		pidMap[pid] = true
	}

	var removed uint32
	oldest := time.Now().Add(-expiry) // yes, t.Add(-d) is the way you do this
	for pid, info := range m {
		if _, ok := pidMap[pid]; ok == false {
			if info.LastSeen.Before(oldest) {
				removed++
				delete(m, pid)
			}
		}
	}
	fmt.Println("pruned", removed, "entries from infoMap")
}

func (p *ProcInfo) init() {
	p.FirstSeen = time.Now()
	p.LastSeen = p.FirstSeen
}

func (p *ProcInfo) touch() {
	p.LastSeen = time.Now()
}

func (p *ProcInfo) updateCmdline() {
	nullSep := []byte{0}
	spaceSep := []byte{32}

	raw, stat, err := ReadSmallFileStat(fmt.Sprintf("/proc/%d/cmdline", p.Pid))
	if err != nil { // proc exited before we could check, or some other even worse problem
		p.Friendly = p.Comm
		return
	}

	// Note this not well documented way to get a UID from a stat
	p.UID = stat.Sys().(*syscall.Stat_t).Uid

	// some things in the process table do not have a command line
	if len(raw) == 0 {
		p.Friendly = p.Comm
		return
	}

	parts := bytes.Split(raw, nullSep)
	// when processes rewrite their argv, we often lose the nulls, split on space instead
	if (len(parts) == 2 && len(parts[1]) == 0) || len(parts) == 1 {
		parts = bytes.Split(parts[0], spaceSep)
	}
	p.Cmdline = make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			p.Cmdline = append(p.Cmdline, string(part))
		}
	}

	pathParts := strings.Split(p.Cmdline[0], "/")
	lastPath := pathParts[len(pathParts)-1]
	switch lastPath {
	case "python":
		p.Friendly = resolvePython(p.Cmdline)
	case "docker":
		p.Friendly = resolveDocker(p.Cmdline)
	case "java":
		p.Friendly = resolveJava(p.Cmdline)
	case "sh", "bash":
		p.Friendly = resolveSh(p.Cmdline)
	case "xargs":
		p.Friendly = resolveXargs(p.Cmdline)
	case "node0.10", "node":
		p.Friendly = resolveNode(p.Cmdline)
	case "uwsgi":
		p.Friendly = resolveUwsgi(p.Cmdline)
	default:
		p.Friendly = resolveDefault(p.Cmdline, p.Comm)
	}

	p.Friendly = strings.Map(StripSpecial, p.Friendly)
}

func resolveUwsgi(parts []string) string {
	argParts := strings.Split(parts[len(parts)-1], "/")
	if len(argParts) > 2 && strings.HasSuffix(argParts[len(argParts)-1], ".json") {
		return argParts[len(argParts)-2]
	}
	return "uwsgi"
}

func resolveNode(parts []string) string {
	if len(parts) <= 1 {
		return "node"
	}
	argParts := strings.Split(parts[1], "/")
	file := argParts[len(argParts)-1]
	if len(file) > 1 {
		return file
	}
	return "node"
}

func resolveXargs(parts []string) string {
	if len(parts) <= 1 {
		return "xargs"
	}

	argParts := strings.Split(parts[len(parts)-1], "/")
	file := argParts[len(argParts)-1]
	if len(file) > 1 {
		return fmt.Sprintf("xargs %s", file)
	}
	return "xargs"
}

func resolveSh(parts []string) string {
	return parts[0]
}

func resolvePython(parts []string) string {
	if len(parts) <= 1 {
		return "python"
	}
	argParts := strings.Split(parts[1], "/")
	file := argParts[len(argParts)-1]
	if len(file) > 1 {
		return file
	}
	return "python"
}

func resolveDocker(parts []string) string {
	if len(parts) <= 1 {
		return "docker"
	}
	argParts := strings.Split(parts[1], "/")
	file := argParts[len(argParts)-1]
	if len(file) > 1 {
		return fmt.Sprintf("docker %s", file)
	}
	return "docker"
}

func resolveJava(parts []string) string {
	if len(parts) <= 1 {
		return "java1"
	}
	for i := 1; i < len(parts); { // start at 1 because 0 is "java"
		if parts[i][0] == byte('-') {
			if parts[i] == "-cp" {
				i += 2
			} else {
				i++
			}
		} else {
			return parts[i]
		}
	}
	return "java2"
}

func resolveDefault(parts []string, comm string) string {
	if strings.Count(parts[0], "/") >= 2 {
		pathParts := strings.Split(parts[0], "/")
		return pathParts[len(pathParts)-1]
	}
	return parts[0]
}
