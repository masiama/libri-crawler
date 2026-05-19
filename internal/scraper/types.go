package scraper

import (
	"context"
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
	Cache  *URLCache
}

type Task struct {
	URL     string
	Type    TaskType
	Handler func(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error)
}

type ScrapedBook struct {
	ISBN       string     `json:"isbn"`
	Title      string     `json:"title"`
	Authors    []string   `json:"authors"`
	URL        string     `json:"url"`
	SourceName SourceName `json:"sourceName"`
	Barcodes   []Barcode  `json:"barcodes,omitempty"`
	ImageURL   string     `json:"-"`
}

type Barcode struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
