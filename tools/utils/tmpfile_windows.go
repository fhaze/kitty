// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package utils

import (
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Create a temporary file that is deleted when its last handle is closed,
// the Windows equivalent of an unlinked open file.
func CreateAnonymousTemp(dir string, perms ...fs.FileMode) (*os.File, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	for try := 0; try < 10000; try++ {
		path := filepath.Join(dir, RandomFilename())
		p, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return nil, err
		}
		h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.CREATE_NEW,
			windows.FILE_ATTRIBUTE_TEMPORARY|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
		if err != nil {
			if err == windows.ERROR_FILE_EXISTS {
				continue
			}
			return nil, &os.PathError{Op: "open", Path: path, Err: err}
		}
		return os.NewFile(uintptr(h), path), nil
	}
	return nil, &os.PathError{Op: "open", Path: dir, Err: fs.ErrExist}
}
