// License: GPLv3 Copyright: 2023, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build !windows

package transfer

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func syscall_mode(i os.FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&os.ModeSetuid != 0 {
		o |= unix.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= unix.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= unix.S_ISVTX
	}
	// No mapping for Go's ModeTemporary (plan9 only).
	return
}

func (self *remote_file) apply_metadata() {
	t := unix.NsecToTimespec(int64(self.mtime))
	for {
		if err := unix.UtimesNanoAt(unix.AT_FDCWD, self.expanded_local_path, []unix.Timespec{t, t}, unix.AT_SYMLINK_NOFOLLOW); err == nil || !(errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN)) {
			break
		}
	}
	if self.ftype == FileType_symlink {
		for {
			if err := unix.Fchmodat(unix.AT_FDCWD, self.expanded_local_path, syscall_mode(self.permissions), unix.AT_SYMLINK_NOFOLLOW); err == nil || !(errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN)) {
				break
			}
		}
	} else {
		_ = os.Chmod(self.expanded_local_path, self.permissions)
	}
}
