package redis

import "libri-crawler/internal/scraper"

type EventType string

const (
	EventBook       EventType = "book"
	EventProgress   EventType = "progress"
	EventCompleted  EventType = "completed"
	EventError      EventType = "error"
	EventCrawlError EventType = "crawl_error"
)

type CrawlerEvent struct {
	Type       EventType            `json:"type"`
	CrawlID    int64                `json:"crawlId"`
	BooksFound *int64               `json:"booksFound,omitempty"`
	Error      *string              `json:"error,omitempty"`
	Book       *scraper.ScrapedBook `json:"book,omitempty"`
}

type crawlerCommand struct {
	CrawlID int64              `json:"crawlId"`
	Source  scraper.SourceName `json:"source"`
}
