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
	TGid                  uint32 // copied from header, should be the same as AcPid
	Version               uint16 // internal, probably 8
	AcExitcode            uint32 // not used until we listen for events
	AcFlag                uint8  // not sure
	AcNice                uint8  // seems like it'd be obvious, but it isn't
	CPUCount              uint64 // delay count waiting for CPU, while runnable
	CPUDelayTotal         uint64 // delay time waiting for CPU, while runnable, in ns
	BlkIOCount            uint64 // delay count waiting for disk
	BlkIODelayTotal       uint64 // delay time waiting for disk
	SwapinCount           uint64 // delay count waiting for swap
	SwapinDelayTotal      uint64 // delay time waiting for swap
	CPURunRealTotal       uint64 // probably the time spent running on CPU, in ns, perhaps adjusted for virt steal
	CPURunVirtualTotal    uint64 // probably the time spent running on CPU, in ns
	AcComm                string // common name, best to ignore this and use /proc/pid/cmdline
	AcSched               uint8  // scheduling discipline, whatever that means
	AcUid                 uint32 // user id
	AcGid                 uint32 // group id
	AcPid                 uint32 // process id, should be the same as TGid, maybe
	AcPPid                uint32 // parent process id
	AcBTime               uint32 // begin time since epoch
	AcETime               uint64 // elapsed total time in us
	AcUTime               uint64 // elapsed user time in us
	AcSTime               uint64 // elapsed system time in us
	AcMinflt              uint64 // major page fault count
	AcMajflt              uint64 // minor page fault count
	Coremem               uint64 // RSS in MBytes/usec
	Virtmem               uint64 // VSZ in MBytes/usec
	HiwaterRSS            uint64 // highest RSS in KB
	HiwaterVM             uint64 // highest VSZ in KB
	ReadChar              uint64 // total bytes read
	WriteChar             uint64 // total bytes written
	ReadSyscalls          uint64 // read system calls
	WriteSyscalls         uint64 // write system calls
	ReadBytes             uint64 // bytes read total
	WriteBytes            uint64 // bytes written total
	CancelledWriteBytes   uint64 // bytes of cancelled write IO, whatever that is
	Nvcsw                 uint64 // voluntary context switches
	Nivcsw                uint64 // involuntary context switches
	AcUTimeScaled         uint64 // user time scaled by CPU frequency
	AcSTimeScaled         uint64 // system time scaled by CPU frequency
	CPUScaledRunRealTotal uint64 // total time scaled by CPU frequency
	FreepagesCount        uint64 // delay count waiting for memory reclaim
	FreepagesDelayTotal   uint64 // delay time waiting for memory reclaim in unknown units
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
	stats.TGid = tgid
	stats.Version = uint16(ts.Version)
	stats.AcExitcode = uint32(ts.Ac_exitcode)
	stats.AcFlag = uint8(ts.Ac_flag)
	stats.AcNice = uint8(ts.Ac_nice)
	stats.CPUCount = uint64(ts.Cpu_count)
	stats.CPUDelayTotal = uint64(ts.Cpu_delay_total)
	stats.BlkIOCount = uint64(ts.Blkio_count)
	stats.BlkIODelayTotal = uint64(ts.Blkio_delay_total)
	stats.SwapinCount = uint64(ts.Swapin_count)
	stats.SwapinDelayTotal = uint64(ts.Swapin_delay_total)
	stats.CPURunRealTotal = uint64(ts.Cpu_run_real_total)
	stats.CPURunVirtualTotal = uint64(ts.Cpu_run_virtual_total)
	stats.AcComm = stringFromBytes(ts.Ac_comm)
	stats.AcSched = uint8(ts.Ac_sched)
	stats.AcUid = uint32(ts.Ac_uid)
	stats.AcPid = uint32(ts.Ac_pid)
	stats.AcPPid = uint32(ts.Ac_ppid)
	stats.AcBTime = uint32(ts.Ac_btime)
	stats.AcETime = uint64(ts.Ac_etime)
	stats.AcUTime = uint64(ts.Ac_utime)
	stats.AcSTime = uint64(ts.Ac_stime)
	stats.AcMinflt = uint64(ts.Ac_minflt)
	stats.AcMajflt = uint64(ts.Ac_majflt)
	stats.Coremem = uint64(ts.Coremem)
	stats.Virtmem = uint64(ts.Virtmem)
	stats.HiwaterRSS = uint64(ts.Hiwater_rss)
	stats.HiwaterVM = uint64(ts.Hiwater_vm)
	stats.ReadChar = uint64(ts.Read_char)
	stats.WriteChar = uint64(ts.Write_char)
	stats.ReadSyscalls = uint64(ts.Read_syscalls)
	stats.WriteSyscalls = uint64(ts.Write_syscalls)
	stats.ReadBytes = uint64(ts.Read_bytes)
	stats.WriteBytes = uint64(ts.Write_bytes)
	stats.CancelledWriteBytes = uint64(ts.Cancelled_write_bytes)
	stats.Nvcsw = uint64(ts.Nvcsw)
	stats.Nivcsw = uint64(ts.Nivcsw)
	stats.AcUTimeScaled = uint64(ts.Ac_utimescaled)
	stats.AcSTimeScaled = uint64(ts.Ac_stimescaled)
	stats.CPUScaledRunRealTotal = uint64(ts.Cpu_scaled_run_real_total)
	stats.FreepagesCount = uint64(ts.Freepages_count)
	stats.FreepagesDelayTotal = uint64(ts.Freepages_delay_total)

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
