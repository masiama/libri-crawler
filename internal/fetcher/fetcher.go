package fetcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

const (
	AcceptHTML       = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"
	AcceptImage      = "image/avif,image/webp,image/apng,image/*,*/*;q=0.8"
	acceptLanguage   = "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7"
	UserAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	maxAttempts      = 4
	retryDelayBase   = 500 * time.Millisecond
	serverErrorDelay = 5 * time.Second

	errorBodySnippetSize = 200
)

type Request struct {
	Client *http.Client
	URL    string
	Accept string
}

func IsBlocked(err error) bool {
	statusErr, ok := errors.AsType[*HTTPStatusError](err)
	return ok && (statusErr.StatusCode == http.StatusForbidden || statusErr.StatusCode == http.StatusTooManyRequests)
}

// Do executes the request with retries.
// Caller must close the returned body.
func (r Request) Do(ctx context.Context) (io.ReadCloser, error) {
	var lastErr error
	var attempts int
	for attempt := range maxAttempts {
		attempts = attempt + 1
		if attempt > 0 {
			if err := r.wait(ctx, attempt, lastErr); err != nil {
				return nil, err
			}
		}

		result, err := r.fetch(ctx)
		if err == nil {
			return result, nil
		}
		if IsBlocked(err) {
			return nil, fmt.Errorf("fetch %s blocked: %w", r.URL, err)
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

	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", r.Accept)
	req.Header.Set("Accept-Language", acceptLanguage)

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		statusErr := &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}
		if resp.StatusCode >= http.StatusInternalServerError || IsBlocked(statusErr) {
			// Server header and body tell an origin failure or a block apart from a CDN/anti-bot response.
			snippet, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodySnippetSize))
			statusErr.Server = resp.Header.Get("Server")
			statusErr.Body = strings.Join(strings.Fields(string(snippet)), " ")
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil, statusErr
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(b)), nil
}

func (r Request) wait(ctx context.Context, attempt int, lastErr error) error {
	factor := 1 << uint(attempt)
	maxBackoff := float64(retryDelayBase * time.Duration(factor))
	sleepDuration := time.Duration(rand.Float64() * maxBackoff)

	if statusErr, ok := errors.AsType[*HTTPStatusError](lastErr); ok {
		switch {
		case statusErr.StatusCode >= http.StatusInternalServerError:
			sleepDuration = serverErrorDelay * time.Duration(attempt)
		}
	}

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
	Server     string
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e.Server == "" && e.Body == "" {
		return fmt.Sprintf("unexpected HTTP response status: %s", e.Status)
	}
	return fmt.Sprintf("unexpected HTTP response status: %s (server: %q, body: %q)", e.Status, e.Server, e.Body)
}
