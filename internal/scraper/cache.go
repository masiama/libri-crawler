package scraper

import "sync"

type URLCache struct {
	mu    sync.RWMutex
	Items map[string]struct{}
}

func (c *URLCache) Seen(url string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.Items[url]; exists {
		return true
	}
	c.Items[url] = struct{}{}
	return false
}
