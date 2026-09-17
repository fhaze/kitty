// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package loop

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/kovidgoyal/kitty/tools/tty"
)

type Signal = syscall.Signal

// Windows only delivers SIGINT (Ctrl+C/Ctrl+Break) and SIGTERM (console
// close) via os/signal. The remaining values exist so that the loop's
// handlers can be expressed uniformly; SIGWINCH is synthesized by the tty
// package from console resize events.
const (
	SIGNULL  Signal = 0
	SIGINT          = syscall.SIGINT
	SIGTERM         = syscall.SIGTERM
	SIGHUP          = syscall.SIGHUP
	SIGPIPE         = syscall.SIGPIPE
	SIGWINCH Signal = 28
	SIGTSTP  Signal = 20
)

func is_temporary_error(err error) bool {
	return errors.Is(err, io.ErrShortWrite) || errors.Is(err, os.ErrDeadlineExceeded)
}

// There is no way to deliver a signal to oneself on Windows, so exit with
// the conventional shell exit status for death by signal instead.
func kill_self(sig Signal) {
	os.Exit(128 + int(sig))
}

func suspend_self() {}

func notify_signals(ch chan os.Signal, term *tty.Term) func() {
	handled_signals := []os.Signal{SIGINT, SIGTERM}
	signal.Notify(ch, handled_signals...)
	term.NotifyResize(ch, SIGWINCH)
	return func() {
		signal.Reset(handled_signals...)
		term.NotifyResize(nil, SIGNULL)
	}
}
