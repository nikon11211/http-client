package httpclient

import (
	"context"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiter *rate.Limiter
}

func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	if config == nil || config.Limit <= 0 {
		return &RateLimiter{}
	}
	burst := config.Burst
	if burst <= 0 {
		burst = 1
	}
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(config.Limit), burst),
	}
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	if r.limiter == nil {
		return nil
	}
	return r.limiter.Wait(ctx)
}
