package downloader

import (
	"context"
	"libri-crawler/internal/fetcher"
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

	data, err := fetcher.Request{Client: d.Client, URL: book.ImageURL}.Do(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = data.Close() }()
	return d.Store.Save(ctx, book, data)
}
