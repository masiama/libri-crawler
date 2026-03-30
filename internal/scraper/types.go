package scraper

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/html"
)

type TaskType int

const (
	TypeDiscovery TaskType = iota
	TypeBook
)

type Scraper struct {
	Client *http.Client
	DB     *pgxpool.Pool
	Cache  *URLCache
}

type Task struct {
	URL     string
	Type    TaskType
	Handler func(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error)
}

type ScrapedBook struct {
	ISBN       string
	Title      string
	Authors    []string
	URL        string
	SourceName SourceName
	ImageURL   string
}
