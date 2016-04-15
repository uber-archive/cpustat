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

// #include <linux/taskstats.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"time"
)

// On older Linux systems, including linux/genetlink.h and taskstats.h doesn't compile.
// To fix this, we define these three symbols here. Note that they just happen to be
// sequential, but they are from 3 different enums.
const (
	CTRL_ATTR_FAMILY_ID   = 1
	CTRL_ATTR_FAMILY_NAME = 2
	CTRL_CMD_GETFAMILY    = 3
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

func readGetTaskstatsMessage(conn *NLConn, task *TaskStats) error {
	inBytes, err := conn.Read()
	if err != nil {
		return err
	}
	if len(inBytes) <= 0 {
		return fmt.Errorf("short read requesting taskstats info: %d bytes", len(inBytes))
	}
	nlmsgs, err := syscall.ParseNetlinkMessage(inBytes)
	if err != nil {
		return err
	}

	if len(nlmsgs) != 1 {
		panic(fmt.Sprint("got unexpected response size from get genl taskstats request: ", len(nlmsgs)))
	}

	if nlmsgs[0].Header.Type == syscall.NLMSG_ERROR {
		var errno int32
		buf := bytes.NewBuffer(nlmsgs[0].Data)
		_ = binary.Read(buf, binary.LittleEndian, &errno)
		if errno == -1 {
			panic("no permission")
		}
		return fmt.Errorf("Netlink error code %d getting taskstats for %d", errno, nlmsgs[0].Header.Pid)
	}

	task.Capturetime = time.Now()
	var offset int
	payload := nlmsgs[0].Data
	endian := binary.LittleEndian

	// these offsets and padding will break if struct taskstats ever changes
	// gen header 0-3
	// attr 4-7
	// attr 8-11
	tgid := endian.Uint32(payload[12:16])
	// attr 16-19

	offset = 20

	offset += 2 // version
	offset += 2 // 2 byte padding
	offset += 4 // exit code
	offset++    // flag
	offset++    // nice
	offset += 6 // 6 byte padding
	task.Cpudelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	task.Cpudelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	task.Blkiodelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	task.Blkiodelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	task.Swapindelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	task.Swapindelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8
	offset += 8  // cpu run real total
	offset += 8  // cpu run virtual total
	offset += 32 // comm
	offset++     // sched
	offset += 7  // 7 byte padding
	offset += 4  // uid
	offset += 4  // gid
	pid := endian.Uint32(payload[offset : offset+4])
	offset += 4
	if pid != tgid {
		fmt.Printf("read value for unexpected pid %d != %d %+v\n", pid, tgid, task)
	}
	offset += 4 // etime
	offset += 4 // btime
	offset += 4 // 4 byte padding
	offset += 8 // etime
	offset += 8 // utime
	offset += 8 // stime
	offset += 8 // minflt
	offset += 8 // majflt
	offset += 8 // coremem
	offset += 8 // virtmem
	offset += 8 // hiwater rss
	offset += 8 // hiwater vsz
	offset += 8 // readchar
	offset += 8 // writechar
	offset += 8 // readsys
	offset += 8 // writesys
	offset += 8 // readbytes
	offset += 8 // writebytes
	offset += 8 // cancelled write bytes
	task.Nvcsw = endian.Uint64(payload[offset : offset+8])
	offset += 8
	task.Nivcsw = endian.Uint64(payload[offset : offset+8])
	offset += 8
	offset += 8 // utimescaled
	offset += 8 // stimescaled
	offset += 8 // cputimescaled
	task.Freepagesdelaycount = endian.Uint64(payload[offset : offset+8])
	offset += 8
	task.Freepagesdelaytotal = endian.Uint64(payload[offset : offset+8])
	offset += 8

	return nil
}

var (
	systemEndianness = binary.LittleEndian
	globalSeq        = uint32(0)
)

// Send a genl taskstats message and hope that Linux doesn't change this layout in the future
func sendGetTaskstatsMessage(conn *NLConn, pid int) error {
	globalSeq++

	// this packet: is nl header(16) + genl header(4) + attribute(8) = 28
	outBytes := make([]byte, 28)

	// NL header
	binary.LittleEndian.PutUint32(outBytes, uint32(syscall.NLMSG_HDRLEN+4+8)) // len: 4 for genl, 8 for attr
	binary.LittleEndian.PutUint16(outBytes[4:], conn.genlFamily)              // type
	binary.LittleEndian.PutUint16(outBytes[6:], syscall.NLM_F_REQUEST)        // flags
	binary.LittleEndian.PutUint32(outBytes[8:], globalSeq)                    // seq
	binary.LittleEndian.PutUint32(outBytes[12:], uint32(conn.pid))            // pid

	// genl header
	outBytes[16] = C.TASKSTATS_CMD_GET      // command
	outBytes[17] = C.TASKSTATS_GENL_VERSION // version
	// 18 and 19 are reserved

	// attribute can be many things, but this one is 8 bytes of pure joy:
	//    len uint16 (always 8)
	//    cmd uint16 (always C.TASKSTATS_CMD_ATTR_PID)
	//    pid uint32 actual pid we want
	binary.LittleEndian.PutUint16(outBytes[20:], 8)
	binary.LittleEndian.PutUint16(outBytes[22:], C.TASKSTATS_CMD_ATTR_PID)
	binary.LittleEndian.PutUint32(outBytes[24:], uint32(pid))

	_, err := conn.Write(outBytes)
	return err
}

func TaskStatsLookupPid(conn *NLConn, sample *ProcSample) error {
	sendGetTaskstatsMessage(conn, sample.Pid)
	return readGetTaskstatsMessage(conn, &sample.Task)
}

func readGetFamilyMessage(conn *NLConn) (uint16, error) {
	inBytes, err := conn.Read()
	if err != nil {
		return 0, err
	}
	if len(inBytes) <= 0 {
		return 0, fmt.Errorf("short read requesting genl family name: %d bytes", len(inBytes))
	}
	nlmsgs, err := syscall.ParseNetlinkMessage(inBytes)
	if err != nil {
		return 0, err
	}

	if len(nlmsgs) != 1 {
		panic(fmt.Sprint("got unexpected response size from get genl family request: ", len(nlmsgs)))
	}

	if nlmsgs[0].Header.Type == syscall.NLMSG_ERROR {
		var errno int32
		buf := bytes.NewBuffer(nlmsgs[0].Data)
		_ = binary.Read(buf, binary.LittleEndian, &errno)
		return 0, fmt.Errorf("Netlink error code %d getting TASKSTATS family id", errno)
	}
	skipLen := binary.LittleEndian.Uint16(nlmsgs[0].Data[4:])
	payloadType := binary.LittleEndian.Uint16(nlmsgs[0].Data[skipLen+8:])
	if payloadType != CTRL_ATTR_FAMILY_ID {
		return 0, fmt.Errorf("Netlink error: got unexpected genl attribute: %d", payloadType)
	}
	genlFamily := binary.LittleEndian.Uint16(nlmsgs[0].Data[skipLen+8+2:])
	return genlFamily, nil
}

// Send a genl taskstats message to get all genl families
func sendGetFamilyCmdMessage(conn *NLConn) error {
	globalSeq++
	genlName := []byte("TASKSTATS")
	genlName = append(genlName, 0, 0, 0)

	// this packet: is nl header(16) + genl header(4) + attribute(16) = 36
	outBytes := make([]byte, 36)

	// NL header
	binary.LittleEndian.PutUint32(outBytes, uint32(syscall.NLMSG_HDRLEN+4+16)) // len: 4 for genl, 16 for attr
	binary.LittleEndian.PutUint16(outBytes[4:], conn.family)                   // type
	binary.LittleEndian.PutUint16(outBytes[6:], syscall.NLM_F_REQUEST)         // flags
	binary.LittleEndian.PutUint32(outBytes[8:], globalSeq)                     // seq
	binary.LittleEndian.PutUint32(outBytes[12:], uint32(conn.pid))             // pid

	// genl header
	outBytes[16] = CTRL_CMD_GETFAMILY       // command
	outBytes[17] = C.TASKSTATS_GENL_VERSION // version
	// 18 and 19 are reserved

	// attribute can be many things, but this one is 8 bytes of pure joy:
	//    len uint16 (always 8)
	//    cmd uint16 (always genl.TASKSTATS_CMD_ATTR_PID)
	//    pid uint32 actual pid we want
	binary.LittleEndian.PutUint16(outBytes[20:], 11+syscall.NLA_HDRLEN)
	binary.LittleEndian.PutUint16(outBytes[22:], CTRL_ATTR_FAMILY_NAME)
	copy(outBytes[24:], genlName)
	_, err := conn.Write(outBytes)
	return err
}

func getGenlFamily(conn *NLConn) uint16 {
	err := sendGetFamilyCmdMessage(conn)
	if err != nil {
		panic(err)
	}
	gfamily, err := readGetFamilyMessage(conn)
	if err != nil {
		panic(err)
	}
	return gfamily
}

// NLConn holds the context necessary to pass around to external callers
type NLConn struct {
	fd         int
	family     uint16
	genlFamily uint16
	addr       syscall.SockaddrNetlink
	pid        int
	readBuf    []byte
}

func (s NLConn) Read() ([]byte, error) {
	n, _, err := syscall.Recvfrom(s.fd, s.readBuf, 0)
	return s.readBuf[:n], os.NewSyscallError("recvfrom", err)
}

func (s NLConn) Write(b []byte) (n int, err error) {
	e := syscall.Sendto(s.fd, b, 0, &s.addr)
	return len(b), os.NewSyscallError("sendto", e)
}

func (s NLConn) Close() error {
	e := syscall.Close(s.fd)
	return os.NewSyscallError("close", e)
}

func (s NLConn) String() string {
	return fmt.Sprintf("fd=%d family=%d genlFamily=%d pid=%d", s.fd, s.family, s.genlFamily, s.pid)
}

// NLInit sets up a new taskstats netlink socket
// All errors are fatal.
func NLInit() *NLConn {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_GENERIC)
	if err != nil {
		panic(os.NewSyscallError("socket", err))
	}
	conn := NLConn{}
	conn.fd = fd
	conn.family = syscall.NETLINK_GENERIC
	conn.addr.Family = syscall.AF_NETLINK
	conn.addr.Pid = 0
	conn.addr.Groups = 0
	conn.pid = os.Getpid()
	conn.readBuf = make([]byte, 4096)
	err = syscall.Bind(fd, &conn.addr)
	if err != nil {
		panic(os.NewSyscallError("bind", err))
	}

	conn.genlFamily = getGenlFamily(&conn)

	return &conn
}
