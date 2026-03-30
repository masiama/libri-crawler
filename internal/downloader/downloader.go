package downloader

import (
	"context"
	"fmt"
	"libri-crawler/internal/scraper"
	"net/http"
)

type Downloader struct {
	Client *http.Client
	Store  *LocalStorage
}

func (d *Downloader) Download(ctx context.Context, book scraper.ScrapedBook) error {
	if d.Store.Exists(ctx, book) {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", book.ImageURL, nil)
	if err != nil {
		return err
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	return d.Store.Save(ctx, book, resp.Body)
}
