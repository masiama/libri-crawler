package scraper

import (
	"context"
	"fmt"
	"libri-crawler/ent"
	"libri-crawler/ent/book"
	"net/http"

	"entgo.io/ent/dialect/sql"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

const (
	PriorityManual    = 0
	PriorityKnigaLv   = 10
	PriorityMnogoknig = 20
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

	builders := make([]*ent.BookCreate, len(books))
	for i, b := range books {
		builders[i] = s.DB.Book.Create().
			SetIsbn(b.Isbn).
			SetTitle(b.Title).
			SetAuthors(b.Authors).
			SetURL(b.URL).
			SetSourcePriority(b.SourcePriority).
			SetSourceName(b.SourceName)
	}

	return s.DB.Book.CreateBulk(builders...).
		OnConflict(
			sql.ConflictColumns(book.FieldIsbn),
			sql.UpdateWhere(sql.P(func(builder *sql.Builder) {
				builder.WriteString("EXCLUDED.source_priority <= books.source_priority")
			})),
		).
		Update(func(u *ent.BookUpsert) {
			u.UpdateTitle()
			u.UpdateAuthors()
			u.UpdateURL()
			u.UpdateSourcePriority()
			u.UpdateSourceName()
		}).
		Exec(ctx)
}

func (s *Scraper) BookExists(ctx context.Context, url string) bool {
	exists, err := s.DB.Book.Query().
		Where(book.URL(url)).
		Exist(ctx)

	if err != nil {
		fmt.Printf("Database check error for %s: %v\n", url, err)
	}

	return exists
}

func (s *Scraper) ShouldProcess(ctx context.Context, t Task) bool {
	if s.Cache.Seen(t.URL) {
		return false
	}

	if t.Type == TypeBook {
		exists, err := s.DB.Book.Query().
			Where(book.URL(t.URL)).
			Exist(ctx)

		if err != nil {
			fmt.Printf("Database check error for %s: %v\n", t.URL, err)
			return true
		}

		return !exists
	}

	return true
}
