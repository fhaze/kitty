//go:build !windows

package utils

import (
	"golang.org/x/sys/unix"
)

const (
	R_OK AccessMode = unix.R_OK
	W_OK AccessMode = unix.W_OK
	X_OK AccessMode = unix.X_OK
)

// Access reports whether the calling process can access path in the
// specified modes, as with access(2).
func Access(path string, mode AccessMode) error {
	return unix.Access(path, uint32(mode))
}
