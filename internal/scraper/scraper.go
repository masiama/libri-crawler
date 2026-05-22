package scraper

import (
	"context"
	"libri-crawler/internal/fetcher"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

func (s *Scraper) Fetch(ctx context.Context, url string) (*html.Node, error) {
	data, err := fetcher.Request{Client: s.Client, URL: url}.Do(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = data.Close() }()
	return htmlquery.Parse(data)
}
