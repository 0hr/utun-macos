package tun_macos

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

/*
// Headers
#include <sys/types.h>
#include <sys/ioctl.h>
#include <sys/sys_domain.h>
#include <sys/kern_control.h>
#include <net/if_utun.h>

*/
import "C"

const (
	ifacePrefix     = "utun"
	uTunControlName = C.UTUN_CONTROL_NAME // net/if_utun.h -> "com.apple.net.utun_control"
)

// ctlInfo
// kern_control.h
/*
#define MAX_KCTL_NAME   96
struct ctl_info {
	u_int32_t   ctl_id;                             // Kernel Controller ID
	char        ctl_name[MAX_KCTL_NAME];            // Kernel Controller Name (a C string)
};
*/

type ctlInfo struct {
	CtlId   uint32
	CtlName [C.MAX_KCTL_NAME]byte
}

// sockAddrCtl
// kern_control.h
/*
	struct sockaddr_ctl {
		u_char      sc_len;     // depends on size of bundle ID string
		u_char      sc_family;    //AF_SYSTEM
		u_int16_t   ss_sysaddr;   // AF_SYS_KERNCONTROL
		u_int32_t   sc_id;       // Controller unique identifier
		u_int32_t   sc_unit;    // Developer private unit number
		u_int32_t   sc_reserved[5];
	};
*/
type sockAddrCtl struct {
	ScLen      uint8
	ScFamily   uint8
	ScSysAddr  uint16
	ScId       uint32
	ScUnit     uint32
	ScReversed uint32
}

func Open(name string) (*os.File, error) {
	after, found := strings.CutPrefix(name, ifacePrefix)

	if !found {
		return nil, fmt.Errorf("interface name must be utun[0-9]")
	}

	unit, err := strconv.Atoi(after)

	if err != nil {
		return nil, fmt.Errorf("interface name must have unit number")
	}

	fd, err := syscall.Socket(syscall.AF_SYSTEM, syscall.SOCK_DGRAM, C.SYSPROTO_CONTROL)

	if err != nil {
		return nil, err
	}
	var errNo syscall.Errno

	var ctlInfo = &ctlInfo{}
	copy(ctlInfo.CtlName[:], []byte(uTunControlName))

	/**
	  if (ioctl(fd(), CTLIOCGINFO, &ctlInfo) == -1)
	        throw utun_error("ioctl(CTLIOCGINFO)");
	*/
	// kern_control.h
	// #define CTLIOCGINFO     _IOWR('N', 3, struct ctl_info)  /* get id from name */
	// ...
	// sys/ioccom.h
	// #define _IOWR(g, n, t)    _IOC(IOC_INOUT,	(g), (n), sizeof(t))
	_, _, errNo = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), C.CTLIOCGINFO, uintptr(unsafe.Pointer(ctlInfo)))
	if errNo != 0 {
		return nil, fmt.Errorf("error while calling SYS_IOCTL ErrNo: %v", errNo)
	}

	sockAddrCtl := &sockAddrCtl{
		ScLen:    uint8(ctlInfo.CtlId),
		ScFamily: syscall.AF_SYSTEM,
		// sys_domain.h
		// #define AF_SYS_CONTROL          2       /* corresponding sub address type */
		ScSysAddr:  C.AF_SYS_CONTROL,
		ScId:       uint32(unit + 1),
		ScReversed: 0,
	}

	/**
	https://go.dev/src/syscall/asm_darwin_amd64.s
	Difference between Syscall and RawSyscall,
	Syscall notifies to runtime for blocking syscall operations and yield CPU-time to another goroutine, coordinates with the scheduler
	Whereas RawSyscall does not notify runtime, does not coordinate with the scheduler.
	So using RawSyscall may block other goroutines from running.
	*/
	/**
		if (connect(fd(), (struct sockaddr *)&sc, sizeof(sc)) == -1)
	       return -1;
	*/
	_, _, errNo = syscall.RawSyscall(syscall.SYS_CONNECT, uintptr(fd), uintptr(unsafe.Pointer(sockAddrCtl)), unsafe.Sizeof(sockAddrCtl))
	if errNo != 0 {
		return nil, fmt.Errorf("error while calling SYS_CONNECT ErrNo: %v", errNo)
	}

	/**
	  // Get iface name of newly created utun dev.
	  char utunname[20];
	  socklen_t utunname_len = sizeof(utunname);
	  if (getsockopt(fd(), SYSPROTO_CONTROL, UTUN_OPT_IFNAME, utunname, &utunname_len))
	      throw utun_error("getsockopt(SYSPROTO_CONTROL)");
	*/
	var uTunName [20]byte

	_, _, errNo = syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		C.SYSPROTO_CONTROL,
		uintptr(fd),
		C.UTUN_OPT_IFNAME,
		uintptr(unsafe.Pointer(&uTunName)),
		unsafe.Sizeof(uTunName),
		0,
	)

	if errNo != 0 {
		return nil, fmt.Errorf("error while calling SYS_GETSOCKOPT ErrNo: %v", errNo)
	}

	// read and write operations on the file will not block the program's execution
	if err = syscall.SetNonblock(fd, true); err != nil {
		return nil, fmt.Errorf("error while setting non-block mode for file")
	}

	return os.NewFile(uintptr(fd), string(uTunName[:])), nil
}
