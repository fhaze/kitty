// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package tool

import "github.com/kovidgoyal/kitty/tools/cli"

// The desktop-ui kitten implements XDG desktop portals over DBus, which do not exist on Windows
func desktop_ui_entry_point(root *cli.Command) {}
