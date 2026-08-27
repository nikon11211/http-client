package httpclient

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type Retryer struct {
	config RetryerConfig
	logger Logger
	rng    *rand.Rand
	rngMu  sync.Mutex
}

func NewRetryer(config RetryerConfig, logger Logger) *Retryer {
	return newRetryer(config, logger, rand.New(rand.NewSource(time.Now().UnixNano())))
}

func newRetryer(config RetryerConfig, logger Logger, rng *rand.Rand) *Retryer {
	if logger == nil {
		logger = NoopLogger{}
	}
	return &Retryer{config: config, logger: logger, rng: rng}
}

func (r *Retryer) Do(ctx context.Context, fn func() (*http.Response, error)) (*http.Response, error) {
	const trace = "retryer.do"
	attempts := r.config.Attempts
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrRetriesExhausted, err)
		}

		resp, err := fn()
		if err == nil && !isRetryableStatus(resp.StatusCode) {
			return resp, nil
		}

		if err != nil {
			lastErr = err
			r.logger.Warn(fmt.Sprintf("(%s) attempt %d/%d failed: %v", trace, attempt, attempts, err))
		} else {
			lastErr = fmt.Errorf("status code %d", resp.StatusCode)
			r.logger.Warn(fmt.Sprintf("(%s) attempt %d/%d received retryable status code %d", trace, attempt, attempts, resp.StatusCode))
			_ = resp.Body.Close()
		}

		if attempt == attempts {
			break
		}

		delay := r.jitter(r.config.Delay)
		r.logger.DebugF("(%s) attempt %d/%d failed, retrying in %s", trace, attempt, attempts, delay)
		if !r.sleep(ctx, delay) {
			return nil, fmt.Errorf("%w: %w", ErrRetriesExhausted, ctx.Err())
		}
	}
	return nil, fmt.Errorf("%w: last error: %w", ErrRetriesExhausted, lastErr)
}

func isRetryableStatus(code int) bool {
	return code >= 500
}

func (r *Retryer) jitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	r.rngMu.Lock()
	f := r.rng.Float64()
	r.rngMu.Unlock()
	return time.Duration(float64(delay) * (0.5 + f/2))
}

func (r *Retryer) sleep(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
