package session

import (
	"context"
	"time"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/log"
)

// loggingDriver wraps a driver with the spec §3 request-level log points:
// request start (provider/model), response (status/latency), errors (the
// decoded provider message flows through the error text). The llm package
// stays logger-free; the wrapper sits in the engine's driverFor.
type loggingDriver struct {
	inner    llm.Driver
	provider string
	model    string
	lg       *log.Logger
}

func (d loggingDriver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error) {
	if d.lg != nil {
		d.lg.Info("llm request start", "provider", d.provider, "model", d.model)
	}
	start := time.Now()
	s, err := d.inner.Stream(ctx, req)
	if d.lg != nil {
		latency := time.Since(start).Milliseconds()
		if err != nil {
			d.lg.Error("llm request error", "provider", d.provider, "model", d.model, "latency_ms", latency, "error", err)
		} else {
			d.lg.Info("llm request", "provider", d.provider, "model", d.model, "status", 200, "latency_ms", latency)
		}
	}
	return s, err
}
