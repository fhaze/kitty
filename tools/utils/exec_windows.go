// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package utils

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

const NAME_MAX = 255

// Windows cannot replace the process image, so emulate exec() by running the
// program with our stdio and exiting with its exit code. Only returns on error.
func Exec(exe string, argv []string, env []string) error {
	cmd := exec.Command(exe)
	if len(argv) > 0 {
		cmd.Args = argv
	}
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}

func DetachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW}
}

func IsTemporarySyscallError(err error) bool {
	return errors.Is(err, windows.WSAEWOULDBLOCK) || errors.Is(err, windows.ERROR_IO_PENDING) || errors.Is(err, windows.ERROR_BUSY)
}

func InterruptSelf() error {
	return windows.GenerateConsoleCtrlEvent(windows.CTRL_C_EVENT, 0)
}

func DupFileTo(f *os.File, fd int) error {
	var std uint32
	switch fd {
	case 0:
		std = windows.STD_INPUT_HANDLE
	case 1:
		std = windows.STD_OUTPUT_HANDLE
	case 2:
		std = windows.STD_ERROR_HANDLE
	default:
		return errors.ErrUnsupported
	}
	p := windows.CurrentProcess()
	var h windows.Handle
	if err := windows.DuplicateHandle(p, windows.Handle(f.Fd()), p, &h, 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return err
	}
	return windows.SetStdHandle(std, h)
}
