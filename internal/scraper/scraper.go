package scraper

import (
	"bytes"
	"context"
	"io"
	"libri-crawler/internal/fetcher"
	"log/slog"

	"github.com/antchfx/htmlquery"
)

const maxBlockRetries = 6

func (s *Scraper) Fetch(ctx context.Context, t Task) ([]byte, error) {
	for blocks := 1; ; blocks++ {
		if err := s.Breaker.Wait(ctx); err != nil {
			return nil, err
		}
		if s.Throttle != nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-s.Throttle:
			}
		}
		data, err := fetcher.Request{Client: s.Client, URL: t.URL, Accept: fetcher.AcceptHTML}.Do(ctx)
		if err == nil {
			s.Breaker.Success()
			defer func() { _ = data.Close() }()
			return io.ReadAll(data)
		}
		if !fetcher.IsBlocked(err) || blocks >= maxBlockRetries {
			return nil, err
		}
		slog.Warn("source_blocked", "url", t.URL, "attempt", blocks, "error", err)
		if abort := s.Breaker.Blocked(); abort != nil {
			return nil, abort
		}
	}
}

func (t Task) Handle(ctx context.Context, data []byte) ([]Task, []ScrapedBook, error) {
	if t.RawHandler != nil {
		return t.RawHandler(ctx, data)
	}
	node, err := htmlquery.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	return t.Handler(ctx, node)
}
