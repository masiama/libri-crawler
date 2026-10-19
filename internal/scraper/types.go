package scraper

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
)

type TaskType int

const (
	TypeDiscovery TaskType = iota
	TypeBook
)

type Scraper struct {
	Client         *http.Client
	Throttle       <-chan time.Time
	EstimatedBooks atomic.Int64
}

type Task struct {
	URL        string
	Type       TaskType
	Handler    func(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error)
	RawHandler func(ctx context.Context, data []byte) ([]Task, []ScrapedBook, error)
}

type ScrapedBook struct {
	ISBN       string               `json:"isbn"`
	Title      string               `json:"title"`
	Authors    []string             `json:"authors"`
	URL        string               `json:"url"`
	SourceName SourceName           `json:"sourceName"`
	Barcodes   []Barcode            `json:"barcodes,omitempty"`
	Images     map[ImageSide]string `json:"-"`
}

type ImageSide string

const (
	SideFront ImageSide = "front"
	SideBack  ImageSide = "back"
	SideSpine ImageSide = "spine"
)

type Barcode struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
