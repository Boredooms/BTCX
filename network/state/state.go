// Package state reports the current network connectivity status for BCTX.
//
// Connectivity is only ever used to decide whether acquisition may run and to
// display an honest status to the operator. The analysis engine never consults
// this package — it always operates on local evidence.
package state

import (
	"context"
	"net"
	"time"
)

// Status is the observed connectivity state.
type Status string

const (
	// Connected means a reachability probe succeeded.
	Connected Status = "CONNECTED"
	// Disconnected means probes failed (no usable network).
	Disconnected Status = "DISCONNECTED"
	// Airgapped means connectivity checks are disabled by policy.
	Airgapped Status = "AIRGAPPED"
)

// Prober determines connectivity. It is injectable so tests can run fully
// offline without touching the real network.
type Prober interface {
	Probe(ctx context.Context) Status
}

// DialProber performs a bounded TCP dial to decide connectivity. This is the
// only place in the analysis-adjacent runtime that opens a socket, and it is
// used purely to show an honest status — never to fetch analysis data.
type DialProber struct {
	// Targets are host:port endpoints tried in order.
	Targets []string
	Timeout time.Duration
}

// DefaultProber returns a prober with conservative defaults.
func DefaultProber() DialProber {
	return DialProber{
		Targets: []string{"1.1.1.1:53", "8.8.8.8:53"},
		Timeout: 800 * time.Millisecond,
	}
}

// Probe attempts each target and returns Connected on the first success.
func (p DialProber) Probe(ctx context.Context) Status {
	d := net.Dialer{Timeout: p.Timeout}
	for _, t := range p.Targets {
		ctx2, cancel := context.WithTimeout(ctx, p.Timeout)
		conn, err := d.DialContext(ctx2, "tcp", t)
		cancel()
		if err == nil {
			_ = conn.Close()
			return Connected
		}
	}
	return Disconnected
}

// AirgapProber always reports Airgapped and never touches the network.
type AirgapProber struct{}

// Probe implements Prober.
func (AirgapProber) Probe(context.Context) Status { return Airgapped }
