package scraper

import (
	"context"
	"libri/ent"
	"libri/ent/book"
	"net/http"

	"entgo.io/ent/dialect/sql"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

const (
	PriorityManual  = 0
	PriorityKnigaLv = 10
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

func (s *Scraper) SaveBook(ctx context.Context, b ScrapedBook) error {
	return s.DB.Book.
		Create().
		SetIsbn(b.Isbn).
		SetTitle(b.Title).
		SetAuthors(b.Authors).
		SetURL(b.URL).
		SetSourcePriority(b.SourcePriority).
		SetSourceName(b.SourceName).
		OnConflict(
			sql.ConflictColumns(book.FieldIsbn),
			sql.UpdateWhere(sql.P(func(builder *sql.Builder) {
				builder.WriteString("EXCLUDED.source_priority <= books.source_priority")
			})),
		).
		Update(func(u *ent.BookUpsert) {
			u.UpdateTitle()
			u.UpdateSourcePriority()
			u.UpdateSourceName()
		}).
		Exec(ctx)

}
