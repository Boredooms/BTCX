package graph

import (
	"context"

	"github.com/bctx/bctx/ml/features"
)

// MetricsAdapter adapts graph.Service to features.GraphMetricsProvider,
// converting graph.Metrics to the feature engine's value type. This keeps the
// features package free of a direct dependency on graph (no import cycle).
type MetricsAdapter struct {
	svc *Service
}

// NewMetricsAdapter wraps a graph service for the feature engine.
func NewMetricsAdapter(svc *Service) *MetricsAdapter {
	return &MetricsAdapter{svc: svc}
}

// WalletMetrics implements features.GraphMetricsProvider.
func (a *MetricsAdapter) WalletMetrics(ctx context.Context, address string, radius int) (features.GraphMetrics, error) {
	m, err := a.svc.WalletMetrics(ctx, address, radius)
	if err != nil {
		return features.GraphMetrics{}, err
	}
	return features.GraphMetrics{
		Degree:                m.Degree,
		FanIn:                 m.FanIn,
		FanOut:                m.FanOut,
		GraphDepth:            m.GraphDepth,
		HopCount:              m.HopCount,
		ChainLength:           m.ChainLength,
		ValueDecay:            m.ValueDecay,
		CounterpartyDiversity: m.CounterpartyDiversity,
	}, nil
}

var _ features.GraphMetricsProvider = (*MetricsAdapter)(nil)
