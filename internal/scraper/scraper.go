package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/antchfx/htmlquery"
	"github.com/jackc/pgx/v5"
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
	return htmlquery.Parse(resp.Body)
}
func (s *Scraper) SaveBatch(ctx context.Context, books []ScrapedBook) error {
	if len(books) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(books))
	for _, b := range books {
		authorsJSON, err := json.Marshal(b.Authors)
		if err != nil {
			return fmt.Errorf("failed to marshal authors for %s: %v", b.ISBN, err)
		}
		rows = append(rows, []any{b.ISBN, b.Title, string(authorsJSON), b.URL, b.SourceName})
	}

	_, err := s.DB.CopyFrom(
		ctx,
		pgx.Identifier{"books"},
		[]string{"isbn", "title", "authors", "url", "source_name"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return s.upsertBatch(ctx, books)
	}
	return nil
}

func (s *Scraper) upsertBatch(ctx context.Context, books []ScrapedBook) error {
	query := `
		INSERT INTO books (isbn, title, authors, url, source_name)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (isbn) DO UPDATE SET
			title       = EXCLUDED.title,
			authors     = EXCLUDED.authors,
			url         = EXCLUDED.url,
			source_name = EXCLUDED.source_name
		WHERE (SELECT priority FROM sources WHERE name = EXCLUDED.source_name)
		      <= (SELECT priority FROM sources WHERE name = books.source_name)
	`

	batch := &pgx.Batch{}
	for _, b := range books {
		authorsJSON, err := json.Marshal(b.Authors)
		if err != nil {
			return fmt.Errorf("failed to marshal authors for %s: %v", b.ISBN, err)
		}
		batch.Queue(query, b.ISBN, b.Title, string(authorsJSON), b.URL, b.SourceName)
	}

	results := s.DB.SendBatch(ctx, batch)
	defer results.Close()

	for range books {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("upsert error: %v", err)
		}
	}

	return nil
}

func (s *Scraper) BookExists(ctx context.Context, url string) (bool, error) {
	var exists bool
	err := s.DB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM books WHERE url = $1)", url,
	).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("database check error for %s: %v", url, err)
	}

	return exists, nil
}
