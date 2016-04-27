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
	"bytes"
	"log"
	"os"
	"strconv"
	"strings"
)

// crude protection against rollover. This will miss the last portion of the previous sample
// before the overflow, but capturing that is complicated because of the various number types
// involved and their inconsistent documentation.
func SafeSub(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return a - b
}

func SafeSubFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return a - b
}

func ScaledSub(cur, prev uint64, scale float64) uint64 {
	return uint64((float64(SafeSub(cur, prev)) * scale) + 0.5)
}

// note that this is not thread safe
var buf *bytes.Buffer

// ReadSmallFile is like os.ReadFile but dangerously optimized for reading files from /proc.
// The file is not statted first, and the same buffer is used every time.
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

// ReadSmallFileStat is like ReadSmallFile except it also returns a FileInfo from os.Stat
func ReadSmallFileStat(filename string) ([]byte, os.FileInfo, error) {
	f, err := os.Open(filename)
	if err != nil {
		f.Close()
		return nil, nil, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}

	if buf == nil {
		buf = bytes.NewBuffer(make([]byte, 0, 8192))
	} else {
		buf.Reset()
	}
	_, err = buf.ReadFrom(f)
	f.Close()
	return buf.Bytes(), info, err
}

// Read a small file and split on newline
func ReadFileLines(filename string) ([]string, error) {
	file, err := ReadSmallFile(filename)
	if err != nil {
		return nil, err
	}

	// TODO - these next two lines cause more GC than I expected
	fileStr := strings.TrimSpace(string(file))
	return strings.Split(fileStr, "\n"), nil
}

// pull a float64 out of a string
func ReadFloat(str string) float64 {
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		log.Fatal(err)

	}
	return val
}

// pull a uint64 out of a string
func ReadUInt(str string) uint64 {
	val, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return val
}

// pull an int64 out of a string
func ReadInt(str string) int64 {
	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	return val
}

// remove grouping characters that confuse the termui parser
func StripSpecial(r rune) rune {
	if r == '[' || r == ']' || r == '(' || r == ')' {
		return -1
	}
	return r
}
