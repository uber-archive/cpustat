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

package cpustat

import (
	"bytes"
	"fmt"
	"strings"
)

type Cmdline struct {
	parts    []string
	friendly string
}

type CmdlineMap map[int]*Cmdline

func updateCmdline(cmds CmdlineMap, pid int, comm string) {
	nullSep := []byte{0}
	spaceSep := []byte{32}

	// if we've seen this before, always use the previous value, even though some programs change
	if _, ok := cmds[pid]; ok == true {
		return
	}

	newCmdline := Cmdline{}
	cmds[pid] = &newCmdline

	raw, err := ReadSmallFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil { // proc exited before we could check, or some other even worse problem
		newCmdline.friendly = comm
		return
	}

	if len(raw) == 0 {
		newCmdline.friendly = comm
		return
	}

	parts := bytes.Split(raw, nullSep)
	// when processes rewrite their argv, we often lose the nulls, split on space instead
	if (len(parts) == 2 && len(parts[1]) == 0) || len(parts) == 1 {
		parts = bytes.Split(parts[0], spaceSep)
	}
	newCmdline.parts = make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			newCmdline.parts = append(newCmdline.parts, string(part))
		}
	}

	pathParts := strings.Split(newCmdline.parts[0], "/")
	lastPath := pathParts[len(pathParts)-1]
	switch lastPath {
	case "python":
		newCmdline.friendly = resolvePython(newCmdline.parts)
	case "docker":
		newCmdline.friendly = resolveDocker(newCmdline.parts)
	case "java":
		newCmdline.friendly = resolveJava(newCmdline.parts)
	case "sh", "bash":
		newCmdline.friendly = resolveSh(newCmdline.parts)
	case "xargs":
		newCmdline.friendly = resolveXargs(newCmdline.parts)
	case "node0.10", "node":
		newCmdline.friendly = resolveNode(newCmdline.parts)
	case "uwsgi":
		newCmdline.friendly = resolveUwsgi(newCmdline.parts)
	default:
		newCmdline.friendly = resolveDefault(newCmdline.parts, comm)
	}

	newCmdline.friendly = strings.Map(StripSpecial, newCmdline.friendly)
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
