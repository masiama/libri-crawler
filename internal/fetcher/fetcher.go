package fetcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	maxAttempts    = 3
	retryDelayBase = 500 * time.Millisecond
)

var retryableFragments = []string{
	"GOAWAY",
	"connection reset by peer",
	"EOF",
	"use of closed network connection",
}

type Request struct {
	Client *http.Client
	URL    string
}

// Do executes the request with retries.
// Caller must close the returned body.
func (r Request) Do(ctx context.Context) (io.ReadCloser, error) {
	var lastErr error
	var attempts int
	for attempt := range maxAttempts {
		attempts = attempt + 1
		if attempt > 0 {
			if err := r.wait(ctx, attempt); err != nil {
				return nil, err
			}
		}

		result, err := r.fetch(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !r.isRetryable(err) {
			break
		}
	}
	return nil, fmt.Errorf("fetch %s failed after %d attempts: %w", r.URL, attempts, lastErr)
}

func (r Request) fetch(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set a User-Agent for azon.market
	req.Header.Set("User-Agent", "libri-crawler")

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil, &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(b)), nil
}

func (r Request) wait(ctx context.Context, attempt int) error {
	factor := 1 << uint(attempt)
	maxBackoff := float64(retryDelayBase * time.Duration(factor))
	sleepDuration := time.Duration(rand.Float64() * maxBackoff)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(sleepDuration):
		return nil
	}
}

func (r Request) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	if httpErr, ok := errors.AsType[*HTTPStatusError](err); ok {
		return httpErr.StatusCode == http.StatusBadGateway ||
			httpErr.StatusCode == http.StatusServiceUnavailable ||
			httpErr.StatusCode == http.StatusGatewayTimeout ||
			httpErr.StatusCode == http.StatusTooManyRequests
	}

	if netErr, ok := errors.AsType[net.Error](err); ok {
		if netErr.Timeout() {
			return true
		}
	}

	msg := err.Error()
	for _, fragment := range retryableFragments {
		if strings.Contains(msg, fragment) {
			return true
		}
	}
	return false
}

type HTTPStatusError struct {
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP response status: %s", e.Status)
}
