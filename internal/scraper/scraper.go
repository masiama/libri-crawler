package scraper

import (
	"bytes"
	"context"
	"io"
	"libri-crawler/internal/fetcher"

	"github.com/antchfx/htmlquery"
)

func (s *Scraper) Fetch(ctx context.Context, t Task) ([]byte, error) {
	if s.Throttle != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.Throttle:
		}
	}
	data, err := fetcher.Request{Client: s.Client, URL: t.URL}.Do(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = data.Close() }()
	return io.ReadAll(data)
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
