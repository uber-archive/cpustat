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

// Integration with go-netlink
// Sending and receiving of taskstats messages is reimplemented here for performance reasons.
// We use go-netlink to fetch the family id and set up the socket.

package main

// #include <linux/taskstats.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"time"

	netlink "github.com/remyoudompheng/go-netlink"
	"github.com/remyoudompheng/go-netlink/genl"
)

type taskStats struct {
	captureTime           time.Time
	prevTime              time.Time
	version               uint16 // internal, probably 8
	exitcode              uint32 // not used until we listen for events
	flag                  uint8  // not sure
	nice                  uint8  // seems like it'd be obvious, but it isn't
	cpudelaycount         uint64 // delay count waiting for CPU, while runnable
	cpudelaytotal         uint64 // delay time waiting for CPU, while runnable, in ns
	blkiodelaycount       uint64 // delay count waiting for disk
	blkiodelaytotal       uint64 // delay time waiting for disk
	swapindelaycount      uint64 // delay count waiting for swap
	swapindelaytotal      uint64 // delay time waiting for swap
	cpurunrealtotal       uint64 // probably the time spent running on CPU, in ns, perhaps adjusted for virt steal
	cpurunvirtualtotal    uint64 // probably the time spent running on CPU, in ns
	comm                  string // common name, best to ignore this and use /proc/pid/cmdline
	sched                 uint8  // scheduling discipline, whatever that means
	uid                   uint32 // user id
	gid                   uint32 // group id
	pid                   uint32 // process id, should be the same as TGid, maybe
	ppid                  uint32 // parent process id
	btime                 uint32 // begin time since epoch
	etime                 uint64 // elapsed total time in us
	utime                 uint64 // elapsed user time in us
	stime                 uint64 // elapsed system time in us
	minflt                uint64 // major page fault count
	majflt                uint64 // minor page fault count
	coremem               uint64 // RSS in MBytes/usec
	virtmem               uint64 // VSZ in MBytes/usec
	hiwaterrss            uint64 // highest RSS in KB
	hiwatervm             uint64 // highest VSZ in KB
	readchar              uint64 // total bytes read
	writechar             uint64 // total bytes written
	readsyscalls          uint64 // read system calls
	writesyscalls         uint64 // write system calls
	readbytes             uint64 // bytes read total
	writebytes            uint64 // bytes written total
	cancelledwritebytes   uint64 // bytes of cancelled write IO, whatever that is
	nvcsw                 uint64 // voluntary context switches
	nivcsw                uint64 // involuntary context switches
	utimescaled           uint64 // user time scaled by CPU frequency
	stimescaled           uint64 // system time scaled by CPU frequency
	cpuscaledrunrealtotal uint64 // total time scaled by CPU frequency
	freepagescount        uint64 // delay count waiting for memory reclaim
	freepagesdelaytotal   uint64 // delay time waiting for memory reclaim in unknown units
}

// convert a byte slice of a null terminated C string into a Go string
func stringFromBytes(c []byte) string {
	nullPos := 0
	i := 0
	for ; i < len(c); i++ {
		if c[i] == 0 {
			nullPos = i
			break
		}
	}
	return string(c[:nullPos])
}

// Because of reflection overhead, the main payload is not read into a big struct.
// It'd probably also be faster to convert the header reading to use the same technique.
func parseResponse(msg syscall.NetlinkMessage) (*taskStats, error) {
	var err error

	buf := bytes.NewBuffer(msg.Data)
	var genHeader netlink.GenlMsghdr
	err = binary.Read(buf, netlink.SystemEndianness, &genHeader)
	if err != nil {
		return nil, err
	}
	var attr syscall.RtAttr

	err = binary.Read(buf, netlink.SystemEndianness, &attr)
	if err != nil {
		return nil, err
	}

	err = binary.Read(buf, netlink.SystemEndianness, &attr)
	if err != nil {
		return nil, err
	}

	var tgid uint32
	err = binary.Read(buf, netlink.SystemEndianness, &tgid)
	if err != nil {
		return nil, err
	}

	err = binary.Read(buf, netlink.SystemEndianness, &attr)
	if err != nil {
		return nil, err
	}

	payload := buf.Bytes()
	offset := 0
	endian := netlink.SystemEndianness
	var stats taskStats
	stats.captureTime = time.Now()

	// these offsets and padding will break if struct taskstats ever changes
	stats.version = endian.Uint16(payload[offset : offset+2])
	offset += 2
	offset += 2 // 2 byte padding
	stats.exitcode = endian.Uint32(payload[offset : offset+4])
	offset += 4
	stats.flag = uint8(payload[offset])
	offset++
	stats.nice = uint8(payload[offset])
	offset++
	offset += 6 // 6 byte padding
	stats.cpudelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.cpudelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.blkiodelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.blkiodelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.swapindelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.swapindelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.cpurunrealtotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.cpurunvirtualtotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.comm = stringFromBytes(payload[offset : offset+32])
	offset += 32
	stats.sched = payload[offset]
	offset++
	offset += 7 // 7 byte padding
	stats.uid = endian.Uint32(payload[offset : offset+4])
	offset += 4
	stats.gid = endian.Uint32(payload[offset : offset+4])
	offset += 4
	stats.pid = endian.Uint32(payload[offset : offset+4])
	offset += 4
	stats.ppid = endian.Uint32(payload[offset : offset+4])
	offset += 4
	stats.btime = endian.Uint32(payload[offset : offset+4])
	offset += 4
	if stats.pid != tgid {
		fmt.Printf("read value for unexpected pid %d != %d %+v\n", stats.pid, tgid, stats)
	}
	offset += 4 // 4 byte padding
	stats.etime = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.utime = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.stime = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.minflt = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.majflt = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.coremem = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.virtmem = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.hiwaterrss = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.hiwatervm = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.readchar = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.writechar = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.readsyscalls = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.writesyscalls = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.readbytes = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.writebytes = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.cancelledwritebytes = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.nvcsw = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.nivcsw = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.utimescaled = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.stimescaled = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.cpuscaledrunrealtotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.freepagescount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.freepagesdelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8

	return &stats, nil
}

type NetlinkError struct {
	msg  string
	Code int32
}

func (e *NetlinkError) Error() string {
	return e.msg
}

func parseError(msg syscall.NetlinkMessage) error {
	var errno int32
	buf := bytes.NewBuffer(msg.Data)
	err := binary.Read(buf, netlink.SystemEndianness, &errno)
	if err != nil {
		return err
	}
	return &NetlinkError{"netlink read", errno}
}

func parseTaskStats(msg syscall.NetlinkMessage) (*taskStats, error) {
	switch msg.Header.Type {
	case syscall.NLMSG_NOOP:
		fmt.Printf("NLMSG_NOOP")
		return nil, nil
	case syscall.NLMSG_ERROR:
		return nil, parseError(msg)
	case syscall.NLMSG_DONE:
		fmt.Printf("NLMSG_DONE\n")
		return nil, nil
	case syscall.NLMSG_OVERRUN:
		fmt.Printf("NLMSG_OVERRUN\n")
	}
	return parseResponse(msg)
}

var (
	systemEndianness = binary.LittleEndian
	globalSeq        = uint32(0)
)

// go-netlink calls os.getpid for every message, which is another wasted system call.
// This is roughly the same code except re-using the pid.
func cmdMessage(family uint16, pid int) (msg netlink.GenericNetlinkMessage) {
	msg.Header.Type = family
	msg.Header.Flags = syscall.NLM_F_REQUEST
	msg.GenHeader.Command = genl.TASKSTATS_CMD_GET
	msg.GenHeader.Version = genl.TASKSTATS_GENL_VERSION
	buf := bytes.NewBuffer([]byte{})
	netlink.PutAttribute(buf, genl.TASKSTATS_CMD_ATTR_PID, uint32(pid))
	msg.Data = buf.Bytes()
	return msg
}

// go-netlink wrote and re-wrote the message, once for genl and again for nl.
// This just writes it once.
func sendCmdMessage(conn *NLConn, pid int) error {
	globalSeq++

	// payload of this message is genl header + a single nl attribute
	attrBuf := bytes.NewBuffer([]byte{})
	netlink.PutAttribute(attrBuf, genl.TASKSTATS_CMD_ATTR_PID, uint32(pid))
	attrBytes := attrBuf.Bytes()

	msg := netlink.GenericNetlinkMessage{}
	// this packet: is nl header(16) + genl header(4) + attribute(8) = 28
	msg.Header.Len = uint32(syscall.NLMSG_HDRLEN + 4 + len(attrBytes))
	msg.Header.Type = conn.family
	msg.Header.Flags = syscall.NLM_F_REQUEST
	msg.Header.Seq = globalSeq
	msg.Header.Pid = uint32(conn.pid)
	msg.GenHeader.Command = genl.TASKSTATS_CMD_GET
	msg.GenHeader.Version = genl.TASKSTATS_GENL_VERSION
	// don't set reserved because it's reserved

	outBuf := bytes.NewBuffer([]byte{})
	binary.Write(outBuf, systemEndianness, msg.Header)
	binary.Write(outBuf, systemEndianness, msg.GenHeader)
	outBuf.Write(attrBytes)

	_, err := conn.sock.Write(outBuf.Bytes())
	return err
}

func taskstatsLookupPid(conn *NLConn, pid int) (*taskStats, error) {
	sendCmdMessage(conn, pid)
	// cmd := cmdMessage(conn.family, pid)
	//	netlink.WriteMessage(conn.sock, &cmd)
	res, err := netlink.ReadMessage(conn.sock)
	if err != nil {
		panic(err)
	}
	parsed, err := parseTaskStats(res)
	if err != nil {
		nerr := err.(*NetlinkError)
		if nerr.Code == -1 {
			panic("No permission")
		} else {
			return nil, &NetlinkError{"proc missing", nerr.Code}
		}
	} else {
		return parsed, nil
	}
}

// NLConn holds the context necessary to pass around to external callers
type NLConn struct {
	family uint16
	sock   *netlink.NetlinkConn
	pid    int
}

// NLInit sets up a new taskstats netlink socket
func NLInit() *NLConn {
	idMap, err := genl.GetFamilyIDs()
	if err != nil {
		panic(err)
	}
	taskstatsGenlName := string(C.TASKSTATS_GENL_NAME)
	family := idMap[taskstatsGenlName]

	sock, err := netlink.DialNetlink("generic", 0)
	if err != nil {
		panic(err)
	}

	return &NLConn{family, sock, os.Getpid()}
}
