package main

// #include <linux/netlink.h>
// #include <linux/genetlink.h>
// #include <linux/taskstats.h>
/*
struct taskstats2 {
	__u16	Version;
	__u32	Ac_exitcode;
	__u8	Ac_flag;
	__u8	Ac_nice;
	__u64	Cpu_count __attribute__((aligned(8)));
	__u64	Cpu_delay_total;
	__u64	Blkio_count;
	__u64	Blkio_delay_total;
	__u64	Swapin_count;
	__u64	Swapin_delay_total;
	__u64	Cpu_run_real_total;
	__u64	Cpu_run_virtual_total;
	char	Ac_comm[TS_COMM_LEN];
	__u8	Ac_sched __attribute__((aligned(8)));
	__u8	Ac_pad[3];
	__u32	Ac_uid __attribute__((aligned(8)));
	__u32	Ac_gid;
	__u32	Ac_pid;
	__u32	Ac_ppid;
	__u32	Ac_btime;
	__u64	Ac_etime __attribute__((aligned(8)));
	__u64	Ac_utime;
	__u64	Ac_stime;
	__u64	Ac_minflt;
	__u64	Ac_majflt;
	__u64	Coremem;
	__u64	Virtmem;
	__u64	Hiwater_rss;
	__u64	Hiwater_vm;
	__u64	Read_char;
	__u64	Write_char;
	__u64	Read_syscalls;
	__u64	Write_syscalls;
	__u64	Read_bytes;
	__u64	Write_bytes;
	__u64	Cancelled_write_bytes;
	__u64  Nvcsw;
	__u64  Nivcsw;
	__u64	Ac_utimescaled;
	__u64	Ac_stimescaled;
	__u64	Cpu_scaled_run_real_total;
	__u64	Freepages_count;
	__u64	Freepages_delay_total;
};
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"syscall"
	"time"

	netlink "github.com/remyoudompheng/go-netlink"
	"github.com/remyoudompheng/go-netlink/genl"
)

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

func stringFromBytes(c [32]C.char) string {
	b := make([]byte, len(c))
	nullPos := 0
	i := 0
	for ; i < len(c); i++ {
		if c[i] == 0 {
			nullPos = i
			break
		}
		b[i] = byte(c[i])
	}
	return string(b[:nullPos])
}

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

	//	This is a version of struct taskstats with the first letters capitalized
	var ts C.struct_taskstats2

	err = binary.Read(buf, netlink.SystemEndianness, &ts)
	if err != nil {
		return nil, err
	}
	//	fmt.Printf("stats: %+v\n", ts)

	var stats taskStats
	stats.captureTime = time.Now()
	stats.version = uint16(ts.Version)
	stats.exitcode = uint32(ts.Ac_exitcode)
	stats.flag = uint8(ts.Ac_flag)
	stats.nice = uint8(ts.Ac_nice)
	stats.cpudelaycount = uint64(ts.Cpu_count)
	stats.cpudelaytotal = uint64(ts.Cpu_delay_total)
	stats.blkiodelaycount = uint64(ts.Blkio_count)
	stats.blkiodelaytotal = uint64(ts.Blkio_delay_total)
	stats.swapindelaycount = uint64(ts.Swapin_count)
	stats.swapindelaytotal = uint64(ts.Swapin_delay_total)
	stats.cpurunrealtotal = uint64(ts.Cpu_run_real_total)
	stats.cpurunvirtualtotal = uint64(ts.Cpu_run_virtual_total)
	stats.comm = stringFromBytes(ts.Ac_comm)
	stats.sched = uint8(ts.Ac_sched)
	stats.uid = uint32(ts.Ac_uid)
	stats.pid = uint32(ts.Ac_pid)
	if stats.pid != tgid {
		panic("read value for unexpected pid")
	}
	stats.ppid = uint32(ts.Ac_ppid)
	stats.btime = uint32(ts.Ac_btime)
	stats.etime = uint64(ts.Ac_etime)
	stats.utime = uint64(ts.Ac_utime)
	stats.stime = uint64(ts.Ac_stime)
	stats.minflt = uint64(ts.Ac_minflt)
	stats.majflt = uint64(ts.Ac_majflt)
	stats.coremem = uint64(ts.Coremem)
	stats.virtmem = uint64(ts.Virtmem)
	stats.hiwaterrss = uint64(ts.Hiwater_rss)
	stats.hiwatervm = uint64(ts.Hiwater_vm)
	stats.readchar = uint64(ts.Read_char)
	stats.writechar = uint64(ts.Write_char)
	stats.readsyscalls = uint64(ts.Read_syscalls)
	stats.writesyscalls = uint64(ts.Write_syscalls)
	stats.readbytes = uint64(ts.Read_bytes)
	stats.writebytes = uint64(ts.Write_bytes)
	stats.cancelledwritebytes = uint64(ts.Cancelled_write_bytes)
	stats.nvcsw = uint64(ts.Nvcsw)
	stats.nivcsw = uint64(ts.Nivcsw)
	stats.utimescaled = uint64(ts.Ac_utimescaled)
	stats.stimescaled = uint64(ts.Ac_stimescaled)
	stats.cpuscaledrunrealtotal = uint64(ts.Cpu_scaled_run_real_total)
	stats.freepagescount = uint64(ts.Freepages_count)
	stats.freepagesdelaytotal = uint64(ts.Freepages_delay_total)

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

func lookupPid(conn *NLConn, pid int) (*taskStats, error) {
	msg := cmdMessage(conn.family, pid)

	netlink.WriteMessage(conn.sock, &msg)
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
// This family thing should really be encapsulated within netlink.NetlinkConn, but it isn't.
type NLConn struct {
	family uint16
	sock   *netlink.NetlinkConn
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

	return &NLConn{family, sock}
}
