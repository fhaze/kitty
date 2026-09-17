//go:build windows

package utils

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const (
	R_OK AccessMode = 4
	W_OK AccessMode = 2
	X_OK AccessMode = 1
)

var executable_extensions = sync.OnceValue(func() map[string]bool {
	pathext := os.Getenv("PATHEXT")
	if pathext == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	ans := make(map[string]bool)
	for ext := range strings.SplitSeq(pathext, ";") {
		if ext = strings.ToLower(strings.TrimSpace(ext)); ext != "" {
			ans[ext] = true
		}
	}
	return ans
})

// Access reports whether path can be accessed in the specified modes.
// Windows has no execute permission bit, so executability is determined by
// the file extension being listed in PATHEXT, as cmd.exe does.
func Access(path string, mode AccessMode) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if mode&W_OK != 0 && s.Mode().Perm()&0o200 == 0 {
		return &os.PathError{Op: "access", Path: path, Err: syscall.EACCES}
	}
	if mode&X_OK != 0 && !s.IsDir() && !executable_extensions()[strings.ToLower(filepath.Ext(path))] {
		return &os.PathError{Op: "access", Path: path, Err: syscall.EACCES}
	}
	return nil
}
