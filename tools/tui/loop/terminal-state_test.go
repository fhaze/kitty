// License: GPLv3 Copyright: 2026, Kovid Goyal, <kovid at kovidgoyal.net>

package loop

import (
	"strings"
	"testing"
)

func TestResetStateEscapeCodesDisableFocusTracking(t *testing.T) {
	opts := TerminalStateOptions{focus_tracking: true}
	codes := opts.ResetStateEscapeCodes()
	reset := FOCUS_TRACKING.EscapeCodeToReset()
	if !strings.Contains(codes, reset) {
		t.Fatalf("focus tracking reset is missing from %q", codes)
	}
	if strings.Index(codes, reset) > strings.Index(codes, RESTORE_PRIVATE_MODE_VALUES) {
		t.Fatalf("focus tracking is reset after private modes are restored in %q", codes)
	}

	opts.focus_tracking = false
	if codes = opts.ResetStateEscapeCodes(); strings.Contains(codes, reset) {
		t.Fatalf("focus tracking reset is present when tracking is disabled in %q", codes)
	}
}
