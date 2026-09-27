package fetcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

const (
	maxAttempts    = 3
	retryDelayBase = 500 * time.Millisecond
)

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
	}
	return nil, fmt.Errorf("fetch %s failed after %d attempts: %w", r.URL, attempts, lastErr)
}

func (r Request) fetch(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

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

type HTTPStatusError struct {
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP response status: %s", e.Status)
}
