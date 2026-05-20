package scraper

import (
	lru "github.com/hashicorp/golang-lru/v2"
)

type URLCache struct {
	items *lru.Cache[string, struct{}]
}

func NewURLCache(size int) *URLCache {
	cache, _ := lru.New[string, struct{}](size)
	return &URLCache{items: cache}
}

func (c *URLCache) Seen(url string) bool {
	ok, _ := c.items.ContainsOrAdd(url, struct{}{})
	return ok
}