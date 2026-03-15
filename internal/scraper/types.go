package scraper

import (
	"context"
	"libri/ent"
	"net/http"

	"golang.org/x/net/html"
)

type Scraper struct {
	Client *http.Client
	DB     *ent.Client
}

type Task struct {
	URL     string
	Handler func(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error)
}

type ScrapedBook struct {
	ent.Book
	ImageURL string
}
