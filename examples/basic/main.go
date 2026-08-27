package main

import (
	"context"
	"fmt"
	"log"
	"time"

	httpclient "github.com/nikon11211/http-client"
)

func main() {
	cfg := &httpclient.Config{
		Env:              "dev",
		Address:          "https://httpbin.org",
		MaxIdleConns:     100,
		RequestTimeout:   30 * time.Second,
		IdleConnTimeout:  90 * time.Second,
		MaxResponseBytes: 1 << 20,
		CircuitBreakerConfig: httpclient.CircuitBreakerConfig{
			Name:        "httpbin",
			MaxRequests: 10,
			Counts:      5,
			Timeout:     30 * time.Second,
		},
		RateLimiterConfig: httpclient.RateLimiterConfig{
			Limit: 20,
			Burst: 5,
		},
		RetryerConfig: httpclient.RetryerConfig{
			Attempts: 3,
			Delay:    500 * time.Millisecond,
		},
		AuthConfig: httpclient.AuthConfig{
			Type: "bearer",
		},
		TLSConfig: httpclient.TLSConfig{
			Enabled: true,
		},
	}

	client, err := httpclient.New(cfg, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, cfg.Address+"/get", nil)
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}

	fmt.Printf("status:     %d (%s)\n", resp.StatusCode, resp.Status)
	fmt.Printf("request id: %s\n", resp.RequestID)
	fmt.Printf("size:       %d bytes\n", resp.Size)
	fmt.Printf("attempts:   %d\n", resp.Attempts)
	fmt.Printf("duration:   %s\n", resp.Duration)
	fmt.Printf("body:       %s\n", resp.Body)
}
