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
	if d.Store.Exists(ctx, book, scraper.SideFront) && d.Store.Exists(ctx, book, scraper.SideBack) && d.Store.Exists(ctx, book, scraper.SideSpine) {
		return nil
	}

	fullSet := book.Images[scraper.SideBack] != "" && book.Images[scraper.SideSpine] != ""
	for side, url := range book.Images {
		if url == "" || (!fullSet && d.Store.Exists(ctx, book, side)) {
			continue
		}
		if err := d.download(ctx, book, side, url); err != nil {
			return err
		}
	}
	return nil
}

func (d *Downloader) download(ctx context.Context, book scraper.ScrapedBook, side scraper.ImageSide, url string) error {
	data, err := fetcher.Request{Client: d.Client, URL: url}.Do(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = data.Close() }()
	return d.Store.Save(ctx, book, side, data)
}
