// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package transfer

import (
	"os"
	"time"
)

func (self *remote_file) apply_metadata() {
	t := time.Unix(0, int64(self.mtime))
	_ = os.Chtimes(self.expanded_local_path, t, t)
	if self.ftype != FileType_symlink {
		_ = os.Chmod(self.expanded_local_path, self.permissions)
	}
}
