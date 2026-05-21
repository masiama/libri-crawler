package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"libri-crawler/internal/scraper"

	"github.com/redis/go-redis/v9"
)

const (
	COMMANDS_QUEUE = "crawler:commands"
	EVENTS_QUEUE   = "crawl:events"
	BOOK_URLS_SET  = "books:existing_urls"
)

type Client struct {
	rdb *redis.Client
}

func NewFromEnv() (*Client, error) {
	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	rdb := redis.NewClient(opt)

	return &Client{rdb}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) ListenForCommands(ctx context.Context) (*crawlerCommand, error) {
	results, err := c.rdb.BLPop(ctx, 0, COMMANDS_QUEUE).Result()
	if err != nil {
		return nil, err
	}

	var cmd crawlerCommand
	if err := json.Unmarshal([]byte(results[1]), &cmd); err != nil {
		return nil, fmt.Errorf("malformed command json: %w", err)
	}

	return &cmd, nil
}

func (c *Client) AcquireCrawlLock(ctx context.Context, source scraper.SourceName, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, c.getLockKey(source), "active", ttl).Result()
}

func (c *Client) ExtendCrawlLock(ctx context.Context, source scraper.SourceName, ttl time.Duration) error {
	return c.rdb.Expire(ctx, c.getLockKey(source), ttl).Err()
}

func (c *Client) ReleaseCrawlLock(ctx context.Context, source scraper.SourceName) error {
	return c.rdb.Del(ctx, c.getLockKey(source)).Err()
}

func (c *Client) IsBookURLKnown(ctx context.Context, bookURL string) (bool, error) {
	return c.rdb.SIsMember(ctx, BOOK_URLS_SET, bookURL).Result()
}

func (c *Client) PublishBook(ctx context.Context, crawlID int64, book scraper.ScrapedBook) error {
	return c.publish(ctx, CrawlerEvent{Type: EventBook, CrawlID: crawlID, Book: &book})
}

func (c *Client) PublishProgress(ctx context.Context, crawlID int64, booksFound int64) error {
	return c.publish(ctx, CrawlerEvent{Type: EventProgress, CrawlID: crawlID, BooksFound: &booksFound})
}

func (c *Client) PublishCompleted(ctx context.Context, crawlID int64, booksFound int64) error {
	return c.publish(ctx, CrawlerEvent{Type: EventCompleted, CrawlID: crawlID, BooksFound: &booksFound})
}

func (c *Client) PublishCancelled(ctx context.Context, crawlID int64, booksFound int64) error {
	return c.publish(ctx, CrawlerEvent{Type: EventCancelled, CrawlID: crawlID, BooksFound: &booksFound})
}

func (c *Client) PublishError(ctx context.Context, crawlID int64, err error) error {
	errStr := err.Error()
	return c.publish(ctx, CrawlerEvent{Type: EventError, CrawlID: crawlID, Error: &errStr})
}

func (c *Client) PublishCrawlError(ctx context.Context, crawlID int64, err error, url *string) error {
	errStr := err.Error()
	return c.publish(ctx, CrawlerEvent{Type: EventCrawlError, CrawlID: crawlID, Error: &errStr, URL: url})
}

func (c *Client) GetCancel(ctx context.Context, source scraper.SourceName) (bool, error) {
	val, err := c.rdb.Get(ctx, fmt.Sprintf("crawl:cancel:%s", source)).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	return val == "1", nil
}

func (c *Client) getLockKey(source scraper.SourceName) string {
	return fmt.Sprintf("lock:crawler:%s", source)
}

func (c *Client) publish(ctx context.Context, event CrawlerEvent) error {
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.rdb.LPush(ctx, EVENTS_QUEUE, jsonBytes).Err()
}
