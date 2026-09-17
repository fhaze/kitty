// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build !windows

package watch

import "golang.org/x/sys/unix"

func signal_kitty_to_reload_config(kitty_pid int) error {
	return unix.Kill(kitty_pid, unix.SIGUSR1)
}
