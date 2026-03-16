package scraper

import (
	"context"
	"libri-crawler/ent"
	"net/http"

	"golang.org/x/net/html"
)

type TaskType int

const (
	TypeDiscovery TaskType = iota
	TypeBook
)

type Scraper struct {
	Client *http.Client
	DB     *ent.Client
	Cache  *URLCache
}

type Task struct {
	URL     string
	Type    TaskType
	Handler func(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error)
}

type ScrapedBook struct {
	ent.Book
	ImageURL string
}
