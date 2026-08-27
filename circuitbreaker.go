package httpclient

import (
	"github.com/sony/gobreaker"
)

type CircuitBreaker struct {
	*gobreaker.CircuitBreaker
}

func NewCircuitBreaker(config *CircuitBreakerConfig, onStateChange func(name string, from, to gobreaker.State)) *CircuitBreaker {
	name := "httpclient"
	if config != nil && config.Name != "" {
		name = config.Name
	}

	settings := gobreaker.Settings{
		Name: name,
	}
	if config == nil {
		return &CircuitBreaker{gobreaker.NewCircuitBreaker(settings)}
	}

	settings.MaxRequests = config.MaxRequests
	settings.Interval = config.Interval
	settings.Timeout = config.Timeout
	if config.Counts > 0 {
		settings.ReadyToTrip = func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= config.Counts
		}
	}
	if onStateChange != nil {
		settings.OnStateChange = onStateChange
	}
	return &CircuitBreaker{gobreaker.NewCircuitBreaker(settings)}
}
