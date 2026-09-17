// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build !windows

package tool

import (
	"github.com/kovidgoyal/kitty/kittens/desktop_ui"
	"github.com/kovidgoyal/kitty/tools/cli"
)

func desktop_ui_entry_point(root *cli.Command) {
	desktop_ui.EntryPoint(root)
}
