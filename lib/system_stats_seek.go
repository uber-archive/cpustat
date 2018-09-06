// Copyright (c) 2018 Uber Technologies, Inc.
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

// data from /proc/stat about the entire system
// This is like system_stats but provides a stateful implementation around
// a single open file descriptor

package cpustat

import (
	"bytes"
	"errors"
	"os"
	"strings"
)

type SystemStatsSeekReader struct {
	statFile   *os.File
	readBuffer *bytes.Buffer
}

func (reader *SystemStatsSeekReader) Initialize() error {
	if reader.statFile != nil {
		return errors.New("System stat file already exists")
	}

	statFile, err := os.Open(StatsPath)
	if err != nil {
		statFile.Close()
		return err
	}

	reader.statFile = statFile
	reader.readBuffer = bytes.NewBuffer(make([]byte, 0, 8192))

	return nil
}

func (reader *SystemStatsSeekReader) ReadStats(cur *SystemStats) error {
	if reader.statFile == nil {
		return errors.New("System stat reader is not initialized")
	}

	_, seekError := reader.statFile.Seek(0, 0)
	if seekError != nil {
		return seekError
	}
	reader.readBuffer.Reset()

	_, readError := reader.readBuffer.ReadFrom(reader.statFile)
	if readError != nil {
		return readError
	}

	// TODO: Known to cause garbage
	lines := strings.Split(strings.TrimSpace(string(reader.readBuffer.Bytes())), "\n")

	// For consistency with current code
	// TODO: read this with 0 garbage
	return SystemStatsReaderFromLines(cur, lines)
}

func (reader *SystemStatsSeekReader) Close() error {
	reader.readBuffer = nil
	if reader.statFile != nil {
		return reader.statFile.Close()
	}
	return nil
}
