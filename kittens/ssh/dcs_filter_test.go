// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

package ssh

import (
	"bytes"
	"fmt"
	"testing"
)

var _ = fmt.Print

func TestDCSFilter(t *testing.T) {
	var out bytes.Buffer
	var got []string
	f := new_dcs_filter(&out, func(msgtype, body string) error {
		got = append(got, msgtype+"|"+body)
		return nil
	})
	feed := func(chunks ...string) {
		for _, c := range chunks {
			n, err := f.Write([]byte(c))
			if err != nil || n != len(c) {
				t.Fatalf("Write(%q) returned %d, %v", c, n, err)
			}
		}
	}
	check := func(expected_out string, expected_dcs ...string) {
		if err := f.Flush(); err != nil {
			t.Fatal(err)
		}
		if out.String() != expected_out {
			t.Fatalf("output: %q != %q", out.String(), expected_out)
		}
		if len(got) != len(expected_dcs) {
			t.Fatalf("dcs: %#v != %#v", got, expected_dcs)
		}
		for i := range got {
			if got[i] != expected_dcs[i] {
				t.Fatalf("dcs: %#v != %#v", got, expected_dcs)
			}
		}
		out.Reset()
		got = nil
	}

	feed("hello \x1b[31mworld\x1b[m")
	check("hello \x1b[31mworld\x1b[m")

	feed("a\x1bP@kitty-ssh|abc=\x1b\\b")
	check("ab", "ssh|abc=")

	// DCS split across writes
	feed("x\x1bP@ki", "tty-ask|na", "me\x1b", "\\y")
	check("xy", "ask|name")

	// non-kitty DCS and other escapes pass through, trailing ESC is held then flushed
	feed("\x1bPq#0\x1b\\\x1b]0;title\x07\x1b")
	check("\x1bPq#0\x1b\\\x1b]0;title\x07\x1b")

	// multiple DCS in one write
	feed("\x1bP@kitty-echo|1\x1b\\mid\x1bP@kitty-print|2\x1b\\")
	check("mid", "echo|1", "print|2")
}
