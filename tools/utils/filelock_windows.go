// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package utils

import (
	"io/fs"
	"os"

	"golang.org/x/sys/windows"
)

const lock_all_bytes = ^uint32(0)

func lock(f *os.File, flags uint32, opname string) error {
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, lock_all_bytes, lock_all_bytes, ol); err != nil {
		return &fs.PathError{Op: opname, Path: f.Name(), Err: err}
	}
	return nil
}

func LockFileShared(f *os.File) error {
	return lock(f, 0, "shared flock()")
}

func LockFileExclusive(f *os.File) error {
	return lock(f, windows.LOCKFILE_EXCLUSIVE_LOCK, "exclusive flock()")
}

func UnlockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	if err := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, lock_all_bytes, lock_all_bytes, ol); err != nil {
		return &fs.PathError{Op: "unlock flock()", Path: f.Name(), Err: err}
	}
	return nil
}
