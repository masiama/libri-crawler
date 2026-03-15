package downloader

import (
	"context"
	"fmt"
	"net/http"

	"libri-crawler/internal/scraper"
)

type Downloader struct {
	Client *http.Client
	Store  Storage
}

func (d *Downloader) Download(ctx context.Context, book scraper.ScrapedBook) error {
	key := book.Isbn + ".jpg"

	if d.Store.Exists(ctx, key) {
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

	return d.Store.Save(ctx, key, resp.Body)
}
