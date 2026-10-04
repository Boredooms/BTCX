package models

import (
	"fmt"
	"strings"

	"github.com/bctx/bctx/sdk"
)

// MonitorState is one of the five mutually exclusive monitoring states a report
// can truthfully describe for a subject.
type MonitorState string

const (
	// MonitorStateNoActivity: a session exists but captured nothing.
	MonitorStateNoActivity MonitorState = "no activity"
	// MonitorStateUnavailable: no monitoring session was ever created.
	MonitorStateUnavailable MonitorState = "monitoring unavailable"
	// MonitorStateDisconnected: provider health degraded/reconnecting or the
	// last poll failed.
	MonitorStateDisconnected MonitorState = "provider disconnected"
	// MonitorStatePaused: the session is explicitly paused.
	MonitorStatePaused MonitorState = "monitoring paused"
	// MonitorStateCoverageGap: gaps were recorded (missed window).
	MonitorStateCoverageGap MonitorState = "coverage gap"
)

// ClassifyMonitorState classifies a monitor session into one of the five
// states and returns a human-readable detail string. A nil session means no
// monitoring was ever started. The precedence is deliberate: a coverage gap or
// disconnect is reported even when the session is otherwise "running", because
// those are the states an analyst must not miss.
func ClassifyMonitorState(s *sdk.MonitorSessionRow) (MonitorState, string) {
	if s == nil {
		return MonitorStateUnavailable, "no monitoring session was created for this subject"
	}

	status := strings.ToUpper(strings.TrimSpace(s.Status))
	health := strings.ToLower(strings.TrimSpace(s.Health))

	// Coverage gap takes precedence: a recorded gap is a factual loss of
	// coverage regardless of current status.
	if s.Gaps > 0 {
		return MonitorStateCoverageGap, fmt.Sprintf(
			"%d coverage gap(s) recorded; %d reconnect(s). Window: started %s, last event %s",
			s.Gaps, s.Reconnects, orNA(s.StartedAt), orNA(s.LastEventAt))
	}

	// Provider disconnected: degraded/reconnecting health or no successful poll
	// after start.
	if health == "degraded" || health == "reconnecting" || health == "down" {
		return MonitorStateDisconnected, fmt.Sprintf(
			"provider health %q; last successful poll %s", s.Health, orNA(s.LastPollOKAt))
	}
	if s.LastPollOKAt == "" && status != "PAUSED" {
		return MonitorStateDisconnected, "no successful provider poll recorded for this session"
	}

	if status == "PAUSED" {
		return MonitorStatePaused, "monitoring session is paused"
	}

	if s.EventsSeen == 0 {
		return MonitorStateNoActivity, "monitoring session active but no events captured"
	}

	return MonitorStateNoActivity, fmt.Sprintf(
		"monitoring session captured %d event(s); no gaps recorded", s.EventsSeen)
}

func orNA(s string) string {
	if strings.TrimSpace(s) == "" {
		return NotAvailable
	}
	return s
}
