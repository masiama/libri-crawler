package scraper

import "sync"

type URLCache struct {
	mu    sync.Mutex
	items map[string]struct{}
}

func NewURLCache(size int) *URLCache {
	return &URLCache{items: make(map[string]struct{}, size)}
}

func (c *URLCache) Seen(url string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.items[url]; exists {
		return true
	}
	c.items[url] = struct{}{}
	return false
}
