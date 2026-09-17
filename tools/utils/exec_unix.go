// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build !windows

package utils

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

const NAME_MAX = unix.NAME_MAX

// Replace the current process with exe. Only returns on error.
func Exec(exe string, argv []string, env []string) error {
	return unix.Exec(exe, argv, env)
}

// Run the process in its own session so that it does not receive signals
// from the controlling terminal of this process
func DetachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

func IsTemporarySyscallError(err error) bool {
	return errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) || errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EBUSY)
}

func InterruptSelf() error {
	return unix.Kill(os.Getpid(), unix.SIGINT)
}

// Make fd refer to the same file as f
func DupFileTo(f *os.File, fd int) error {
	return unix.Dup2(int(f.Fd()), fd)
}
