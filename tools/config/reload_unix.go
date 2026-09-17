// License: GPLv3 Copyright: 2023, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build !windows

package config

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kovidgoyal/kitty/tools/utils"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/unix"
)

func ReloadConfigInKitty(in_parent_only bool) error {
	if in_parent_only {
		if pid, err := strconv.ParseInt(os.Getenv("KITTY_PID"), 10, 32); err == nil {
			if p, err := process.NewProcess(int32(pid)); err == nil {
				if exe, eerr := p.Exe(); eerr == nil {
					if c, err := p.CmdlineSlice(); err == nil && is_kitty_gui_cmdline(exe, c...) {
						return p.SendSignal(unix.SIGUSR1)
					}
				}
			}
		}
		return nil
	}
	// process.Processes() followed by filtering by getting the process
	// exe and cmdline is very slow on non-Linux systems as CGO is not allowed
	// which means getting exe works by calling lsof on every process. So instead do
	// initial filtering based on ps output.
	if ps_out, err := exec.Command("ps", "-x", "-o", "pid=,comm=").Output(); err == nil {
		for _, line := range utils.Splitlines(utils.UnsafeBytesToString(ps_out)) {
			line = strings.TrimSpace(line)
			if pid_string, argv0, found := strings.Cut(line, " "); found {
				if pid, err := strconv.ParseInt(strings.TrimSpace(pid_string), 10, 32); err == nil && strings.Contains(argv0, "kitty") {
					if p, err := process.NewProcess(int32(pid)); err == nil {
						if cmdline, err := p.CmdlineSlice(); err == nil {
							if exe, err := p.Exe(); err == nil && is_kitty_gui_cmdline(exe, cmdline...) {
								_ = p.SendSignal(unix.SIGUSR1)
							}
						}
					}
				}
			}
		}
	}
	return nil
}
