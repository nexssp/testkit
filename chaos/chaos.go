package chaos

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/kernel/xerr"
)

type Config struct {
	Enabled   bool
	ErrorRate float64
	PanicRate float64
	MaxDelay  time.Duration
}

// Inject returns an action.Middleware that applies stochastic latency, errors, or panics.
func Inject[Req, Res any](cfg Config) action.Middleware[Req, Res] {
	return func(next action.Fn[Req, Res]) action.Fn[Req, Res] {
		return func(ctx context.Context, req Req) (Res, error) {
			if err := applyChaos(ctx, cfg); err != nil {
				var zero Res
				return zero, err
			}
			return next(ctx, req)
		}
	}
}

// GlobalHook returns an AnyHook applying chaos across all actions in a test suite.
func GlobalHook(cfg Config) action.AnyHook {
	return action.AnyHook{
		Before: func(ctx context.Context, _ any, _ *action.Meta) (context.Context, error) {
			return ctx, applyChaos(ctx, cfg)
		},
	}
}

func applyChaos(ctx context.Context, cfg Config) error {
	if !cfg.Enabled {
		return nil
	}

	if cfg.PanicRate > 0 && rand.Float64() < cfg.PanicRate { //nolint:gosec // test-only chaos
		panic("chaos: simulated panic injection")
	}

	if cfg.MaxDelay > 0 {
		delay := time.Duration(rand.Float64() * float64(cfg.MaxDelay)) //nolint:gosec // test-only chaos
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	if cfg.ErrorRate > 0 && rand.Float64() < cfg.ErrorRate { //nolint:gosec // test-only chaos
		return xerr.Unavailable("chaos: simulated transient failure")
	}

	return nil
}
