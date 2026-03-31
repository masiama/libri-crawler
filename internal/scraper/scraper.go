package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"libri-crawler/internal/api"
	"net/http"
	"net/url"

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

func (s *Scraper) SaveBatch(ctx context.Context, books []ScrapedBook) error {
	if len(books) == 0 {
		return nil
	}

	resp, err := s.API.Post(ctx, "/api/v1/internal/books/batch", map[string][]ScrapedBook{"books": books})
	if err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("batch rejected with status %d: %s", resp.StatusCode, api.ReadError(resp))
	}

	return nil
}

func (s *Scraper) BookExists(ctx context.Context, bookURL string) (bool, error) {
	resp, err := s.API.Get(ctx, "/api/v1/internal/books/exists?url="+url.QueryEscape(bookURL))
	if err != nil {
		return false, fmt.Errorf("failed to check book existence: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var res struct {
		Exists bool `json:"exists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return false, err
	}
	return res.Exists, nil
}
