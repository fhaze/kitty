//go:build !windows

package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func macos_user_cache_dir() string {
	// Sadly Go does not provide confstr() so we use this hack.
	// Note that given a user generateduid and uid we can derive this by using
	// the algorithm at https://github.com/ydkhatri/MacForensics/blob/master/darwin_path_generator.py
	// but I cant find a good way to get the generateduid. Requires calling dscl in which case we might as well call getconf
	// The data is in /var/db/dslocal/nodes/Default/users/<username>.plist but it needs root
	// So instead we use various hacks to get it quickly, falling back to running /usr/bin/getconf

	is_ok := func(m string) bool {
		s, err := os.Stat(m)
		if err != nil {
			return false
		}
		stat, ok := s.Sys().(syscall.Stat_t)
		return ok && s.IsDir() && int(stat.Uid) == os.Geteuid() && s.Mode().Perm() == 0o700 && Access(m, X_OK|W_OK|R_OK) == nil
	}

	if tdir := strings.TrimRight(os.Getenv("TMPDIR"), "/"); filepath.Base(tdir) == "T" {
		if m := filepath.Join(filepath.Dir(tdir), "C"); is_ok(m) {
			return m
		}
	}

	matches, err := filepath.Glob("/private/var/folders/*/*/C")
	if err == nil {
		for _, m := range matches {
			if is_ok(m) {
				return m
			}
		}
	}
	out, err := exec.Command("/usr/bin/getconf", "DARWIN_USER_CACHE_DIR").Output()
	if err == nil {
		return strings.TrimRight(strings.TrimSpace(UnsafeBytesToString(out)), "/")
	}
	return ""
}
