// License: GPLv3 Copyright: 2022, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build !windows

package loop

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"time"

	"golang.org/x/sys/unix"

	"github.com/kovidgoyal/kitty/tools/tty"
)

type Signal = unix.Signal

const (
	SIGNULL  Signal = 0
	SIGINT          = unix.SIGINT
	SIGTERM         = unix.SIGTERM
	SIGHUP          = unix.SIGHUP
	SIGWINCH        = unix.SIGWINCH
	SIGPIPE         = unix.SIGPIPE
	SIGTSTP         = unix.SIGTSTP
)

func is_temporary_error(err error) bool {
	return errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, io.ErrShortWrite)
}

func kill_self(sig Signal) {
	_ = unix.Kill(os.Getpid(), sig)
	// Give the signal time to be delivered
	time.Sleep(20 * time.Millisecond)
}

func suspend_self() {
	_ = unix.Kill(os.Getpid(), unix.SIGSTOP)
	time.Sleep(20 * time.Millisecond)
}

// Deliver the signals the loop handles on ch, returning a function that undoes this
func notify_signals(ch chan os.Signal, term *tty.Term) func() {
	handled_signals := []os.Signal{SIGINT, SIGTERM, SIGTSTP, SIGHUP, SIGWINCH, SIGPIPE}
	signal.Notify(ch, handled_signals...)
	return func() { signal.Reset(handled_signals...) }
}
