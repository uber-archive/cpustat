package main

import (
	"bytes"
	"log"
	"os"
	"strconv"
	"strings"
)

// crude protection against rollover. This will miss the last portion of the previous sample
// before the overflow, but capturing that is complicated because of the various number types
// involved and their inconsistent documentation.
func safeSub(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return a - b
}

func safeSubFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return a - b
}

func scaledSub(cur, prev uint64, scale float64) uint64 {
	return uint64((float64(safeSub(cur, prev)) * scale) + 0.5)
}

// note that this is not thread safe
var buf *bytes.Buffer

// ReadSmallFile is like os.ReadFile but skips the stat
func ReadSmallFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		f.Close()
		return nil, err
	}

	if buf == nil {
		buf = bytes.NewBuffer(make([]byte, 0, 8192))
	} else {
		buf.Reset()
	}
	_, err = buf.ReadFrom(f)
	f.Close()
	return buf.Bytes(), err
}

func readFileLines(filename string) ([]string, error) {
	file, err := ReadSmallFile(filename)
	if err != nil {
		return nil, err
	}

	fileStr := strings.TrimSpace(string(file))
	return strings.Split(fileStr, "\n"), nil
}

func readFloat(str string) float64 {
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		log.Fatal(err)

	}
	return val
}

func readUInt(str string) uint64 {
	val, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return val
}

func readInt(str string) int64 {
	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	return val
}

func stripSpecial(r rune) rune {
	if r == '[' || r == ']' || r == '(' || r == ')' {
		return -1
	}
	return r
}
