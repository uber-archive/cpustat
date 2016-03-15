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

// data from /proc/stat about the entire system
// This data appears to be updated somewhat frequently
package cpustat

import (
	"bytes"
	"log"
	"strings"
	"time"
)

type SystemStats struct {
	CaptureTime  time.Time
	Usr          uint64
	Nice         uint64
	Sys          uint64
	Idle         uint64
	Iowait       uint64
	Irq          uint64
	Softirq      uint64
	Steal        uint64
	Guest        uint64
	GuestNice    uint64
	Ctxt         uint64
	ProcsTotal   uint64
	ProcsRunning uint64
	ProcsBlocked uint64
}

func writeUint64LE(buf *bytes.Buffer, num uint64) {
	buf.WriteByte(byte(num))
	buf.WriteByte(byte(num >> 8))
	buf.WriteByte(byte(num >> 16))
	buf.WriteByte(byte(num >> 24))
	buf.WriteByte(byte(num >> 32))
	buf.WriteByte(byte(num >> 40))
	buf.WriteByte(byte(num >> 48))
	buf.WriteByte(byte(num >> 56))
}

func (s *SystemStats) writeBuf(buf *bytes.Buffer) {
	tbuf, _ := s.CaptureTime.MarshalBinary()
	buf.Write(tbuf)
	writeUint64LE(buf, s.Usr)
	writeUint64LE(buf, s.Nice)
	writeUint64LE(buf, s.Sys)
	writeUint64LE(buf, s.Idle)
	writeUint64LE(buf, s.Iowait)
	writeUint64LE(buf, s.Irq)
	writeUint64LE(buf, s.Softirq)
	writeUint64LE(buf, s.Steal)
	writeUint64LE(buf, s.Guest)
	writeUint64LE(buf, s.GuestNice)
	writeUint64LE(buf, s.Ctxt)
	writeUint64LE(buf, s.ProcsTotal)
	writeUint64LE(buf, s.ProcsRunning)
	writeUint64LE(buf, s.ProcsBlocked)
}

func readUint64LE(buf *bytes.Buffer) uint64 {
	b0, err0 := buf.ReadByte()
	b1, err1 := buf.ReadByte()
	b2, err2 := buf.ReadByte()
	b3, err3 := buf.ReadByte()
	b4, err4 := buf.ReadByte()
	b5, err5 := buf.ReadByte()
	b6, err6 := buf.ReadByte()
	b7, err7 := buf.ReadByte()

	if err0 != nil || err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil ||
		err7 != nil {
		return 0
	}

	return uint64(b0) | uint64(b1)<<8 | uint64(b2)<<16 | uint64(b3)<<24 | uint64(b4)<<32 | uint64(b5)<<40 |
		uint64(b6)<<48 | uint64(b7)<<56
}

const sizeTime = 15

func (sys *SystemStats) readBuf(buf *bytes.Buffer) {
	raw := make([]byte, sizeTime)
	var t time.Time

	buf.Read(raw)
	t.UnmarshalBinary(raw)
	sys.CaptureTime = t

	sys.Usr = readUint64LE(buf)
	sys.Nice = readUint64LE(buf)
	sys.Sys = readUint64LE(buf)
	sys.Idle = readUint64LE(buf)
	sys.Iowait = readUint64LE(buf)
	sys.Irq = readUint64LE(buf)
	sys.Softirq = readUint64LE(buf)
	sys.Steal = readUint64LE(buf)
	sys.Guest = readUint64LE(buf)
	sys.GuestNice = readUint64LE(buf)
	sys.Ctxt = readUint64LE(buf)
	sys.ProcsTotal = readUint64LE(buf)
	sys.ProcsRunning = readUint64LE(buf)
	sys.ProcsBlocked = readUint64LE(buf)
}

func SystemStatsReader() *SystemStats {
	lines, err := ReadFileLines("/proc/stat")
	if err != nil {
		log.Fatal("reading /proc/stat: ", err)
	}

	cur := SystemStats{}

	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), " ")
		switch parts[0] {
		case "cpu":
			cur.CaptureTime = time.Now()

			parts = parts[1:] // global cpu line has an extra space for some human somewhere
			cur.Usr = ReadUInt(parts[1])
			cur.Nice = ReadUInt(parts[2])
			cur.Sys = ReadUInt(parts[3])
			cur.Idle = ReadUInt(parts[4])
			cur.Iowait = ReadUInt(parts[5])
			cur.Irq = ReadUInt(parts[6])
			cur.Softirq = ReadUInt(parts[7])
			cur.Steal = ReadUInt(parts[8])
			cur.Guest = ReadUInt(parts[9])
			cur.GuestNice = ReadUInt(parts[10])
		case "ctxt":
			cur.Ctxt = ReadUInt(parts[1])
		case "processes":
			cur.ProcsTotal = ReadUInt(parts[1])
		case "procs_running":
			cur.ProcsRunning = ReadUInt(parts[1])
		case "procs_blocked":
			cur.ProcsBlocked = ReadUInt(parts[1])
		default:
			continue
		}
	}

	return &cur
}

func SystemStatsRecord(interval int, cur, prev, sum *SystemStats) *SystemStats {
	delta := &SystemStats{}

	sum.CaptureTime = cur.CaptureTime
	delta.CaptureTime = cur.CaptureTime
	duration := float64(cur.CaptureTime.Sub(prev.CaptureTime) / time.Millisecond)
	scale := float64(interval) / duration

	delta.Usr = ScaledSub(cur.Usr, prev.Usr, scale)
	sum.Usr += SafeSub(cur.Usr, prev.Usr)
	delta.Nice = ScaledSub(cur.Nice, prev.Nice, scale)
	sum.Nice += SafeSub(cur.Nice, prev.Nice)
	delta.Sys = ScaledSub(cur.Sys, prev.Sys, scale)
	sum.Sys += SafeSub(cur.Sys, prev.Sys)
	delta.Idle = ScaledSub(cur.Idle, prev.Idle, scale)
	sum.Idle += SafeSub(cur.Idle, prev.Idle)
	delta.Iowait = ScaledSub(cur.Iowait, prev.Iowait, scale)
	sum.Iowait += SafeSub(cur.Iowait, prev.Iowait)
	delta.Irq = ScaledSub(cur.Irq, prev.Irq, scale)
	sum.Irq += SafeSub(cur.Irq, prev.Irq)
	delta.Softirq = ScaledSub(cur.Softirq, prev.Softirq, scale)
	sum.Softirq += SafeSub(cur.Softirq, prev.Softirq)
	delta.Steal = ScaledSub(cur.Steal, prev.Steal, scale)
	sum.Steal += SafeSub(cur.Steal, prev.Steal)
	delta.Guest = ScaledSub(cur.Guest, prev.Guest, scale)
	sum.Guest += SafeSub(cur.Guest, prev.Guest)
	delta.GuestNice = ScaledSub(cur.GuestNice, prev.GuestNice, scale)
	sum.GuestNice += SafeSub(cur.GuestNice, prev.GuestNice)
	delta.Ctxt = ScaledSub(cur.Ctxt, prev.Ctxt, scale)
	sum.Ctxt += SafeSub(cur.Ctxt, prev.Ctxt)
	delta.ProcsTotal = ScaledSub(cur.ProcsTotal, prev.ProcsTotal, scale)
	sum.ProcsTotal += SafeSub(cur.ProcsTotal, prev.ProcsTotal)
	sum.ProcsRunning = cur.ProcsRunning
	sum.ProcsBlocked = cur.ProcsBlocked

	return delta
}
