// License: GPLv3 Copyright: 2026, Kovid Goyal, <kovid at kovidgoyal.net>

package loop

import (
	"testing"
	"time"
)

func new_capabilities_test_loop(t *testing.T) (*Loop, *int) {
	t.Helper()
	lp := new_loop()
	lp.timers = make([]*timer, 0)
	calls := 0
	lp.OnCapabilitiesReceived = func(TerminalCapabilities) error {
		calls++
		return nil
	}
	lp.QueryCapabilities()
	return lp, &calls
}

func TestCapabilitiesResponseOrdering(t *testing.T) {
	t.Run("keyboard protocol before device attributes", func(t *testing.T) {
		lp, calls := new_capabilities_test_loop(t)
		if err := lp.handle_csi([]byte("?29u")); err != nil {
			t.Fatal(err)
		}
		if err := lp.handle_csi([]byte("?1;2c")); err != nil {
			t.Fatal(err)
		}
		if *calls != 1 {
			t.Fatalf("expected one callback, got %d", *calls)
		}
	})

	t.Run("device attributes before keyboard protocol", func(t *testing.T) {
		lp, calls := new_capabilities_test_loop(t)
		if err := lp.handle_csi([]byte("?1;2c")); err != nil {
			t.Fatal(err)
		}
		if *calls != 0 {
			t.Fatalf("callback ran before the keyboard protocol response")
		}
		if err := lp.handle_csi([]byte("?29u")); err != nil {
			t.Fatal(err)
		}
		if *calls != 1 {
			t.Fatalf("expected one callback, got %d", *calls)
		}
		if len(lp.timers) != 0 {
			t.Fatalf("capability timeout was not cancelled")
		}
	})

	t.Run("terminal without keyboard protocol", func(t *testing.T) {
		lp, calls := new_capabilities_test_loop(t)
		if err := lp.handle_csi([]byte("?1;2c")); err != nil {
			t.Fatal(err)
		}
		if err := lp.dispatch_timers(time.Now().Add(capabilities_response_grace_period + time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		if *calls != 1 {
			t.Fatalf("expected one callback after the grace period, got %d", *calls)
		}
		if err := lp.handle_csi([]byte("?29u")); err != nil {
			t.Fatal(err)
		}
		if *calls != 1 {
			t.Fatalf("late response caused a second callback")
		}
	})
}
