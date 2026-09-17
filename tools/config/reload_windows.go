// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package config

import "errors"

// Windows has no SIGUSR1, config reload must be triggered via remote control
func ReloadConfigInKitty(in_parent_only bool) error {
	return errors.ErrUnsupported
}
