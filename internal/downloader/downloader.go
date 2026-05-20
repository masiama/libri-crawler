package downloader

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"libri-crawler/internal/scraper"
)

type Downloader struct {
	Client *http.Client
	Store  *LocalStorage
}

func (d *Downloader) Download(ctx context.Context, book scraper.ScrapedBook) error {
	if d.Store.Exists(ctx, book) {
		return nil
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", book.ImageURL, nil)
		if err != nil {
			return err
		}

		resp, err := d.Client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return d.Store.Save(ctx, book, resp.Body)
			}
			lastErr = fmt.Errorf("bad status: %s", resp.Status)
			
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return lastErr
			}
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("download failed after 3 attempts: %w", lastErr)
}