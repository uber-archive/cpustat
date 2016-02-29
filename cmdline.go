package main

import (
	"bytes"
	"fmt"
	"strings"
)

type cmdline struct {
	parts    []string
	friendly string
}

type cmdlineMap map[int]*cmdline

func stripSpecial(r rune) rune {
	if r == '[' || r == ']' || r == '(' || r == ')' {
		return -1
	}
	return r
}

func updateCmdline(cmds cmdlineMap, pid int, comm string) {
	nullSep := []byte{0}
	spaceSep := []byte{32}

	// if we've seen this before, always use the previous value, even though some programs change
	if _, ok := cmds[pid]; ok == true {
		return
	}

	newCmdline := cmdline{}
	cmds[pid] = &newCmdline

	raw, err := ReadSmallFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil { // proc exited before we could check, or some other even worse problem
		fmt.Println("updateCmdline readfile err", err)
		newCmdline.friendly = comm
		return
	}

	if len(raw) == 0 {
		newCmdline.friendly = comm
		return
	}

	parts := bytes.Split(raw, nullSep)
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
	default:
		newCmdline.friendly = resolveDefault(newCmdline.parts, comm)
	}

	newCmdline.friendly = strings.Map(stripSpecial, newCmdline.friendly)
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
		return fmt.Sprintf("xargs %s, %s", parts[0], file)
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
