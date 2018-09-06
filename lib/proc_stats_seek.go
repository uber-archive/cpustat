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

package cpustat

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

type ProcStatsSeekReader struct {
	PID        int
	procFile   *os.File
	readBuffer *bytes.Buffer
}

func (reader *ProcStatsSeekReader) Initialize() error {
	if reader.procFile != nil {
		return fmt.Errorf("Proc file for pid %d already exists", reader.PID)
	}

	procFile, err := os.Open(fmt.Sprintf("/proc/%d/stat", reader.PID))
	if err != nil {
		procFile.Close()
		return err
	}

	reader.procFile = procFile
	reader.readBuffer = bytes.NewBuffer(make([]byte, 0, 8192))

	return nil
}

func (reader *ProcStatsSeekReader) ReadStats(cur *ProcStats) error {
	if reader.procFile == nil {
		return fmt.Errorf("Proc stat reader for pid %d is not initialized", reader.PID)
	}

	_, seekError := reader.procFile.Seek(0, 0)
	if seekError != nil {
		return seekError
	}

	reader.readBuffer.Reset()

	_, err := reader.readBuffer.ReadFrom(reader.procFile)
	if err != nil {
		return err
	}

	// TODO: Known to cause garbage
	lines := strings.Split(strings.TrimSpace(string(reader.readBuffer.Bytes())), "\n")
	parts := procPidStatSplit(lines[0])

	// For consistency with current code
	// TODO: read this with 0 garbage
	procStatsReaderFromParts(cur, parts)
	return nil
}

func (reader *ProcStatsSeekReader) Close() error {
	reader.readBuffer = nil
	if reader.procFile != nil {
		return reader.procFile.Close()
	}
	return nil
}
