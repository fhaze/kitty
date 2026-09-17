// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package desktop_ui

import "github.com/kovidgoyal/kitty/tools/cli"

// This kitten implements XDG desktop portals over DBus, which do not exist on Windows
func specialize_command(parent *cli.Command) {
	parent.Run = func(cmd *cli.Command, args []string) (int, error) {
		cmd.ShowHelp()
		return 1, nil
	}
	parent.ShortDescription = "Implement various desktop components for use with lightweight compositors/window managers on Linux"
	parent.Hidden = true
}
