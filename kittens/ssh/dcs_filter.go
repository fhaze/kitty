// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

package ssh

import (
	"bytes"
	"fmt"
	"io"
	"runtime"

	"github.com/kovidgoyal/kitty/tools/utils"
)

var _ = fmt.Print

const dcs_prefix = "\x1bP@kitty-"
const max_dcs_size = 8 * 1024 * 1024

// dcs_filter extracts @kitty-* DCS escape codes from a byte stream, passing
// all other bytes through to output. It is used on Windows where ConPTY does
// not deliver unknown DCS codes to the terminal, so they are instead sent to
// kitty out-of-band via on_dcs.
type dcs_filter struct {
	output io.Writer
	on_dcs func(msgtype, body string) error
	// bytes that might be the start of a @kitty DCS, held back until we know
	pending []byte
}

func new_dcs_filter(output io.Writer, on_dcs func(msgtype, body string) error) *dcs_filter {
	return &dcs_filter{output: output, on_dcs: on_dcs}
}

func (f *dcs_filter) dispatch(dcs []byte) {
	// dcs is the payload after the prefix and before ST
	msgtype, body, found := bytes.Cut(dcs, []byte{'|'})
	if !found {
		return
	}
	_ = f.on_dcs(string(msgtype), utils.UnsafeBytesToString(body))
}

func (f *dcs_filter) write_output(b []byte) error {
	for len(b) > 0 {
		n, err := f.output.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func (f *dcs_filter) Write(input []byte) (int, error) {
	err := f.process(input)
	if err != nil {
		return 0, err
	}
	return len(input), nil
}

func (f *dcs_filter) process(input []byte) error {
	data := input
	if len(f.pending) > 0 {
		data = append(f.pending, input...)
		f.pending = nil
	}
	for len(data) > 0 {
		esc := bytes.IndexByte(data, 0x1b)
		if esc < 0 {
			return f.write_output(data)
		}
		if err := f.write_output(data[:esc]); err != nil {
			return err
		}
		data = data[esc:]
		// data now starts with ESC
		if len(data) < len(dcs_prefix) {
			if bytes.HasPrefix([]byte(dcs_prefix), data) {
				f.pending = append([]byte(nil), data...)
				return nil
			}
			if err := f.write_output(data[:1]); err != nil {
				return err
			}
			data = data[1:]
			continue
		}
		if !bytes.HasPrefix(data, []byte(dcs_prefix)) {
			if err := f.write_output(data[:1]); err != nil {
				return err
			}
			data = data[1:]
			continue
		}
		end := bytes.Index(data[len(dcs_prefix):], []byte("\x1b\\"))
		if end < 0 {
			if len(data) > max_dcs_size {
				// unterminated DCS, give up on it and pass it through
				return f.write_output(data)
			}
			f.pending = append([]byte(nil), data...)
			return nil
		}
		f.dispatch(data[len(dcs_prefix) : len(dcs_prefix)+end])
		data = data[len(dcs_prefix)+end+2:]
	}
	return nil
}

// Flush writes out any held back bytes, used when the stream ends
func (f *dcs_filter) Flush() error {
	if len(f.pending) > 0 {
		p := f.pending
		f.pending = nil
		return f.write_output(p)
	}
	return nil
}

func use_dcs_channel_proxy() bool {
	return runtime.GOOS == "windows" && utils.DCSChannelAddress() != ""
}

func forward_dcs_to_kitty(msgtype, body string) error {
	return utils.SendDCSViaChannel(msgtype, body)
}
