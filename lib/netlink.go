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

package cpustat

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
func parseResponse(msg syscall.NetlinkMessage) (*TaskStats, string, error) {
	var err error

	buf := bytes.NewBuffer(msg.Data)
	var genHeader netlink.GenlMsghdr
	err = binary.Read(buf, netlink.SystemEndianness, &genHeader)
	if err != nil {
		return nil, "", err
	}
	var attr syscall.RtAttr

	err = binary.Read(buf, netlink.SystemEndianness, &attr)
	if err != nil {
		return nil, "", err
	}

	err = binary.Read(buf, netlink.SystemEndianness, &attr)
	if err != nil {
		return nil, "", err
	}

	var tgid uint32
	err = binary.Read(buf, netlink.SystemEndianness, &tgid)
	if err != nil {
		return nil, "", err
	}

	err = binary.Read(buf, netlink.SystemEndianness, &attr)
	if err != nil {
		return nil, "", err
	}

	payload := buf.Bytes()
	offset := 0
	endian := netlink.SystemEndianness
	var stats TaskStats
	stats.Capturetime = time.Now()

	// these offsets and padding will break if struct taskstats ever changes
	offset += 2 // version
	offset += 2 // 2 byte padding
	offset += 4 // exit code
	offset++    // flag
	offset++    // nice
	offset += 6 // 6 byte padding
	stats.Cpudelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Cpudelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Blkiodelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Blkiodelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Swapindelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Swapindelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	offset += 8 // cpu run real total
	offset += 8 // cpu run virtual total
	comm := stringFromBytes(payload[offset : offset+32])
	offset += 32 // comm
	offset++     // sched
	offset += 7  // 7 byte padding
	offset += 4  // uid
	offset += 4  // gid
	pid := endian.Uint32(payload[offset : offset+4])
	offset += 4
	if pid != tgid {
		fmt.Printf("read value for unexpected pid %d != %d %+v\n", pid, tgid, stats)
	}
	offset += 4 // etime
	offset += 4 // btime
	offset += 4 // 4 byte padding
	offset += 8 // etime
	offset += 8 // utime
	offset += 8 // stime
	stats.Minflt = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Majflt = endian.Uint64(payload[offset : offset+8])
	offset += 8
	offset += 8 // coremem
	offset += 8 // virtmem
	offset += 8 // hiwater rss
	offset += 8 // hiwater vsz
	stats.Readchar = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Writechar = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Readsyscalls = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Writesyscalls = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Readbytes = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Writebytes = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Cancelledwritebytes = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Nvcsw = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Nivcsw = endian.Uint64(payload[offset : offset+8])
	offset += 8
	offset += 8 // utimescaled
	offset += 8 // stimescaled
	offset += 8 // cputimescaled
	stats.Freepagesdelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	stats.Freepagesdelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8

	return &stats, comm, nil
}

// NetlinkError should only be used in this file, is duplicated from go-netlink
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

func parseTaskStats(msg syscall.NetlinkMessage) (*TaskStats, string, error) {
	// print a warning if we got an unexpected message type
	switch msg.Header.Type {
	case syscall.NLMSG_NOOP:
		fmt.Printf("NLMSG_NOOP")
		return nil, "", nil
	case syscall.NLMSG_ERROR:
		return nil, "", parseError(msg)
	case syscall.NLMSG_DONE:
		fmt.Printf("NLMSG_DONE\n")
		return nil, "", nil
	case syscall.NLMSG_OVERRUN:
		fmt.Printf("NLMSG_OVERRUN\n")
	}
	return parseResponse(msg)
}

var (
	systemEndianness = binary.LittleEndian
	globalSeq        = uint32(0)
)

// go-netlink wrote and re-wrote the message, once for genl and again for nl.
// This just writes it once, and skips the expensive attr encoding.
// eventually this should completely replace go-netlink
func sendCmdMessage(conn *NLConn, pid int) error {
	globalSeq++

	// payload of this message is genl header + a single nl attribute
	attrBytes := make([]byte, 8)
	attrBytes[0] = 8
	attrBytes[1] = 0
	binary.LittleEndian.PutUint16(attrBytes[2:], genl.TASKSTATS_CMD_ATTR_PID)
	binary.LittleEndian.PutUint32(attrBytes[4:], uint32(pid))

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

func TaskStatsLookupPid(conn *NLConn, pid int) (*TaskStats, string, error) {
	sendCmdMessage(conn, pid)
	// cmd := cmdMessage(conn.family, pid)
	//	netlink.WriteMessage(conn.sock, &cmd)
	res, err := netlink.ReadMessage(conn.sock)
	if err != nil {
		panic(err)
	}
	parsed, comm, err := parseTaskStats(res)
	if err != nil {
		nerr := err.(*NetlinkError)
		if nerr.Code == -1 {
			panic("No permission")
		} else {
			return nil, "", &NetlinkError{"proc missing", nerr.Code}
		}
	} else {
		return parsed, comm, nil
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
