package pty

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The ioctls of grantpt, unlockpt and ptsname on macOS (sys/ttycom.h).
const (
	tiocptygrant = 0x20007454 // _IO('t', 84)
	tiocptyunlk  = 0x20007452 // _IO('t', 82)
	tiocptygname = 0x40807453 // _IOC(IOC_OUT, 't', 83, 128)
)

// Layout of vnode_info_path and proc_vnodepathinfo in <sys/proc_info.h>.
type vnodeInfoPath struct {
	_    [19]uint64 // vnode_info
	path [1024]byte // MAXPATHLEN
}

type procVnodePathInfo struct {
	cwd  vnodeInfoPath
	root vnodeInfoPath
}

var procPIDInfo = sync.OnceValue(func() func(int32, int32, uint64, *procVnodePathInfo, int32) int32 {
	var call func(int32, int32, uint64, *procVnodePathInfo, int32) int32
	if symbol, err := purego.Dlsym(purego.RTLD_DEFAULT, "proc_pidinfo"); err == nil {
		purego.RegisterFunc(&call, symbol)
	}
	return call
})

func (p *PTY) WorkingDirectory() string {
	const procPIDVnodePathInfo = 9 // PROC_PIDVNODEPATHINFO
	call := procPIDInfo()
	if call == nil {
		return ""
	}
	var info procVnodePathInfo
	size := int32(unsafe.Sizeof(info))
	if call(int32(p.Pid()), procPIDVnodePathInfo, 0, &info, size) != size {
		return ""
	}
	path, _, _ := bytes.Cut(info.cwd.path[:], []byte{0})
	return string(path)
}

func openMaster() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("pty: %w", err)
	}
	var name [128]byte
	for _, step := range []struct {
		req uintptr
		arg unsafe.Pointer
	}{{tiocptygrant, nil}, {tiocptyunlk, nil}, {tiocptygname, unsafe.Pointer(&name[0])}} {
		if err := ioctl(master, step.req, step.arg); err != nil {
			master.Close()
			return nil, "", fmt.Errorf("pty: %w", err)
		}
	}
	n := bytes.IndexByte(name[:], 0)
	if n < 0 {
		n = len(name)
	}
	return master, string(name[:n]), nil
}
