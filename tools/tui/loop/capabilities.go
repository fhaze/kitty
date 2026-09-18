package loop

import (
	"fmt"
	"time"
)

var _ = fmt.Print

type ColorPreference uint8

const (
	NO_COLOR_PREFERENCE ColorPreference = iota
	DARK_COLOR_PREFERENCE
	LIGHT_COLOR_PREFERENCE
)

func (c ColorPreference) String() string {
	switch c {
	case DARK_COLOR_PREFERENCE:
		return "dark"
	case LIGHT_COLOR_PREFERENCE:
		return "light"
	default:
		return "no-preference"
	}
}

type TerminalCapabilities struct {
	KeyboardProtocol                 bool
	KeyboardProtocolResponseReceived bool

	ColorPreference                 ColorPreference
	ColorPreferenceResponseReceived bool
}

const capabilities_response_grace_period = 250 * time.Millisecond

func (self *Loop) finish_capabilities_query() error {
	if !self.capabilities_query_pending {
		return nil
	}
	self.capabilities_query_pending = false
	if self.capabilities_timeout_timer_id != 0 {
		self.RemoveTimer(self.capabilities_timeout_timer_id)
		self.capabilities_timeout_timer_id = 0
	}
	if self.OnCapabilitiesReceived != nil {
		return self.OnCapabilitiesReceived(self.TerminalCapabilities)
	}
	return nil
}

func (self *Loop) handle_capabilities_da_response() error {
	if !self.capabilities_query_pending {
		return nil
	}
	self.capabilities_da_response_received = true
	if self.TerminalCapabilities.KeyboardProtocolResponseReceived {
		return self.finish_capabilities_query()
	}
	timer_id, err := self.AddTimer(capabilities_response_grace_period, false, func(IdType) error {
		self.capabilities_timeout_timer_id = 0
		return self.finish_capabilities_query()
	})
	if err != nil {
		return err
	}
	self.capabilities_timeout_timer_id = timer_id
	return nil
}
