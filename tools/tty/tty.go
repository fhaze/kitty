// License: GPLv3 Copyright: 2022, Kovid Goyal, <kovid at kovidgoyal.net>

package tty

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/emmansun/base64"

	"github.com/kovidgoyal/kitty/tools/utils"
)

const (
	TCSANOW   = 0
	TCSADRAIN = 1
	TCSAFLUSH = 2
)

func (self *Term) set_termios_attrs(when uintptr, modify func(*Termios)) (err error) {
	var state Termios
	if err = self.Tcgetattr(&state); err != nil {
		return
	}
	new_state := state
	modify(&new_state)
	if err = self.Tcsetattr(when, &new_state); err == nil {
		self.states = append(self.states, state)
	}
	return
}

func (self *Term) ApplyOperations(when uintptr, operations ...TermiosOperation) (err error) {
	if len(operations) == 0 {
		return
	}
	return self.set_termios_attrs(when, func(t *Termios) {
		for _, op := range operations {
			op(t)
		}
	})
}

func (self *Term) PopStateWhen(when uintptr) (err error) {
	if len(self.states) == 0 {
		return nil
	}
	idx := len(self.states) - 1
	if err = self.Tcsetattr(when, &self.states[idx]); err == nil {
		self.states = self.states[:idx]
	}
	return
}

func (self *Term) PopState() error {
	return self.PopStateWhen(TCSAFLUSH)
}

func (self *Term) RestoreWhen(when uintptr) (err error) {
	if len(self.states) == 0 {
		return nil
	}
	self.states = self.states[:1]
	return self.PopStateWhen(when)
}

func (self *Term) Restore() error {
	return self.RestoreWhen(TCSAFLUSH)
}

func (self *Term) RestoreAndClose() error {
	_ = self.Restore()
	return self.Close()
}

func (self *Term) Suspend() (resume func() error, err error) {
	var state Termios
	err = self.Tcgetattr(&state)
	if err != nil {
		return nil, err
	}
	if len(self.states) > 0 {
		err := self.Tcsetattr(TCSANOW, &self.states[0])
		if err != nil {
			return nil, err
		}
	}
	return func() error { return self.Tcsetattr(TCSANOW, &state) }, nil

}

func (self *Term) SuspendAndRun(callback func() error) error {
	resume, err := self.Suspend()
	if err != nil {
		return err
	}
	err = callback()
	if rerr := resume(); rerr != nil {
		err = rerr
	}
	return err
}

func clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (self *Term) WriteAll(b []byte) error {
	for len(b) > 0 {
		n, err := self.Write(b)
		if err != nil && !is_temporary_error(err) {
			return err
		}
		b = b[n:]
	}
	return nil
}

func (self *Term) WriteAllString(s string) error {
	return self.WriteAll(utils.UnsafeStringToBytes(s))
}

func (self *Term) DebugPrintln(a ...any) {
	msg := fmt.Appendln(nil, a...)
	const limit = 2048
	encoded := make([]byte, limit*2)
	for i := 0; i < len(msg); i += limit {
		end := min(i+limit, len(msg))
		chunk := msg[i:end]
		encoded = encoded[:cap(encoded)]
		base64.StdEncoding.Encode(encoded, chunk)
		_, _ = self.WriteString("\x1bP@kitty-print|")
		_, _ = self.Write(encoded)
		_, _ = self.WriteString("\x1b\\")
	}
}

var KittyStdout = sync.OnceValue(func() *os.File {
	if fds := os.Getenv(`KITTY_STDIO_FORWARDED`); fds != "" {
		if fd, err := strconv.Atoi(fds); err == nil && fd > -1 {
			if f := os.NewFile(uintptr(fd), "<kitty_stdout>"); f != nil {
				return f
			}
		}
	}
	return nil
})

func DebugPrintln(a ...any) {
	if f := KittyStdout(); f != nil {
		fmt.Fprintln(f, a...)
		return
	}
	term, err := OpenControllingTerm()
	if err == nil {
		defer term.Close()
		term.DebugPrintln(a...)
	}
}

func ReadSingleByteFromTerminal() (b byte, err error) {
	term, err := OpenControllingTerm(SetBlockingRead, SetNoCanonical)
	if err != nil {
		return 0, err
	}
	defer term.Close()
	ans := []byte{b}
	for {
		n, err := term.Read(ans)
		if err != nil {
			return 0, err
		}
		if n > 0 {
			return ans[0], nil
		}
	}
}
