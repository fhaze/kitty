// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package watch

import "errors"

func signal_kitty_to_reload_config(kitty_pid int) error {
	return errors.ErrUnsupported
}
