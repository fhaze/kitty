// License: GPLv3 Copyright: 2023, Kovid Goyal, <kovid at kovidgoyal.net>

package tui

import (
	"fmt"

	"github.com/emmansun/base64"

	"github.com/kovidgoyal/kitty/tools/utils"
)

var _ = fmt.Print

// KittyDCS returns the escape code that delivers the DCS @kitty-<msgtype>|<body>
// to kitty, wrapped for tmux passthrough if needed. When kitty has provided an
// out-of-band channel the message is sent over it and an empty string is
// returned, as nothing needs to be written to the tty.
func KittyDCS(msgtype, body string) (string, error) {
	if utils.DCSChannelAddress() != "" {
		return "", utils.SendDCSViaChannel(msgtype, body)
	}
	ans := "\x1bP@kitty-" + msgtype + "|" + body
	tmux := TmuxSocketAddress()
	if tmux != "" {
		err := TmuxAllowPassthrough()
		if err != nil {
			return "", err
		}
		ans = "\033Ptmux;\033" + ans + "\033\033\\\033\\"
	} else {
		ans += "\033\\"
	}
	return ans, nil
}

func DCSToKitty(msgtype, payload string) (string, error) {
	return KittyDCS(msgtype, base64.StdEncoding.EncodeToString(utils.UnsafeStringToBytes(payload)))
}
