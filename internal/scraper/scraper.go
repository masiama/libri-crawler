package scraper

import (
	"context"
	"fmt"
	"net/http"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

func (s *Scraper) Fetch(ctx context.Context, url string) (*html.Node, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch failed: %s", resp.Status)
	}
	return htmlquery.Parse(resp.Body)
}
