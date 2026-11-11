package scraper

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var ErrBlocked = errors.New("source keeps blocking requests, giving up")

// Breaker pauses all workers when a source starts refusing requests, and gives up if it keeps doing so.
type Breaker struct {
	Threshold int
	Pause     time.Duration
	MaxPauses int

	mu      sync.Mutex
	blocked int
	pauses  int
	until   time.Time
}

func (b *Breaker) Wait(ctx context.Context) error {
	if b == nil {
		return ctx.Err()
	}
	b.mu.Lock()
	d := time.Until(b.until)
	b.mu.Unlock()
	if d <= 0 {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func (b *Breaker) Success() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocked, b.pauses = 0, 0
}

func (b *Breaker) Blocked() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if time.Now().Before(b.until) {
		return nil
	}
	b.blocked++
	if b.blocked < b.Threshold {
		return nil
	}
	b.blocked = 0
	b.pauses++
	if b.pauses > b.MaxPauses {
		return ErrBlocked
	}
	pause := b.Pause * time.Duration(b.pauses)
	b.until = time.Now().Add(pause)
	slog.Warn("source_blocked_pausing", "pause", pause.String(), "pauses_in_a_row", b.pauses)
	return nil
}
