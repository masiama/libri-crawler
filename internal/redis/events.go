package redis

type Event string

const (
	EventCompleted = Event("COMPLETED")
	EventError     = Event("ERROR")
)

type crawlCompletedEvent struct {
	Type    Event  `json:"type"`
	CrawlID string `json:"crawlId"`
	Total   int64  `json:"total"`
}

type crawlErrorEvent struct {
	Type    Event  `json:"type"`
	CrawlID string `json:"crawlId"`
	Error   error  `json:"error"`
}
