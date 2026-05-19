package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"libri-crawler/internal/scraper"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewFromEnv() (*Client, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is not set")
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	return &Client{rdb}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) IsBookURLKnown(ctx context.Context, bookURL string) (bool, error) {
	reply, err := c.rdb.SIsMember(ctx, "books:existing_urls", bookURL).Result()
	if err != nil {
		return false, err
	}
	return reply, nil
}

func (c *Client) PublishBook(ctx context.Context, book scraper.ScrapedBook) error {
	jsonBytes, err := json.Marshal(book)
	if err != nil {
		return err
	}

	c.rdb.LPush(ctx, "books:queue", jsonBytes)
	return nil
}

func (c *Client) PublishCompleted(ctx context.Context, crawlID string, total int64) error {
	event := crawlCompletedEvent{
		Type:    EventCompleted,
		CrawlID: crawlID,
		Total:   total,
	}
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	c.rdb.LPush(ctx, "crawl:events", jsonBytes)
	return nil
}

func (c *Client) PublishError(ctx context.Context, crawlID string, err error) error {
	event := crawlErrorEvent{
		Type:    EventError,
		CrawlID: crawlID,
		Error:   err,
	}
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	c.rdb.LPush(ctx, "crawl:events", jsonBytes)
	return nil
}

func (c *Client) PublishProgress(ctx context.Context, crawlID string, count int64) error {
	key := fmt.Sprintf("crawl:%s:progress", crawlID)
	return c.rdb.Set(ctx, key, count, 0).Err()
}
