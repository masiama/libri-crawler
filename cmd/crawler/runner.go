package main

import (
	"context"
	"errors"
	"fmt"
	"libri-crawler/internal/downloader"
	"libri-crawler/internal/redis"
	"libri-crawler/internal/scraper"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	scraperWorkers    = 10
	downloaderWorkers = 100
	publisherWorkers  = 3
)

type Runner struct {
	HTTPClient *http.Client
	Store      *downloader.LocalStorage
	Redis      *redis.Client
}

func (r *Runner) Run(ctx context.Context, source scraper.SourceName, crawlID int64) error {
	start := time.Now()
	slog.Debug(string(LogEventCrawlStarted), "source", source, "crawl_id", crawlID)

	var totalProcessed atomic.Int64
	var cancelled atomic.Bool

	s := &scraper.Scraper{Client: r.HTTPClient}
	dl := &downloader.Downloader{Store: r.Store, Client: r.HTTPClient}

	rootTask, ok := sourceTasks(s)[source]
	if !ok {
		return errors.New(string(LogEventInvalidSource))
	}

	var wg sync.WaitGroup
	var scraperWg sync.WaitGroup
	var activeTasks sync.WaitGroup
	recordGlobalErr := func(err error, event LogEvent, url *string) {
		if err == nil {
			return
		}
		_ = r.Redis.PublishCrawlError(ctx, crawlID, err, url)
		slog.Error(string(event), "source", source, "crawl_id", crawlID, "error", err, "url", url)
	}

	tasksChan := make(chan scraper.Task, 10_000)
	publishChan := make(chan scraper.ScrapedBook, 20_000)
	imagesChan := make(chan scraper.ScrapedBook, 50_000)

	reportCtx, cancelReport := context.WithCancel(ctx)
	defer cancelReport()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var lastReported int64 = -1
		for {
			select {
			case <-reportCtx.Done():
				return
			case <-ticker.C:
				count := totalProcessed.Load()
				if count == lastReported {
					continue
				}
				lastReported = count
				slog.Debug(string(LogEventCrawlProgress), "source", source, "books_found", count)
				err := r.Redis.PublishProgress(ctx, crawlID, count)
				if err != nil {
					slog.Error(string(LogEventCrawlProgressPublishFailed), "source", source, "crawl_id", crawlID, "error", err, "books_found", count)
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-reportCtx.Done():
				return
			case <-ticker.C:
				if err := r.Redis.PublishHeartbeat(ctx, crawlID); err != nil {
					slog.Error(string(LogEventHeartbeatPublishFailed), "crawl_id", crawlID, "error", err)
				}
			}
		}
	}()

	for range publisherWorkers {
		wg.Go(func() {
			for book := range publishChan {
				err := r.Redis.PublishBook(ctx, crawlID, book)
				if err != nil {
					recordGlobalErr(err, LogEventBookPublishFailed, &book.URL)
				}
			}
		})
	}

	for range downloaderWorkers {
		wg.Go(func() {
			for b := range imagesChan {
				if err := dl.Download(ctx, b); err != nil {
					recordGlobalErr(err, LogEventImageDownloadFailed, &b.URL)
				}
			}
		})
	}

	for range scraperWorkers {
		scraperWg.Add(1)
		wg.Go(func() {
			defer scraperWg.Done()

			for t := range tasksChan {
				if c, _ := r.Redis.GetCancel(ctx, source); c {
					cancelled.Store(true)
					activeTasks.Done()
					continue
				}

				node, err := s.Fetch(ctx, t.URL)
				if err != nil {
					recordGlobalErr(err, LogEventSourceFetchFailed, &t.URL)
					activeTasks.Done()
					continue
				}

				next, books, _ := t.Handler(ctx, node)

				totalProcessed.Add(int64(len(books)))

				for _, b := range books {
					publishChan <- b
					imagesChan <- b
				}

				activeTasks.Add(1)
				go func(tasksToAdd []scraper.Task) {
					defer activeTasks.Done()
					for _, nt := range tasksToAdd {
						seen, err := r.Redis.SeenURL(ctx, crawlID, nt.URL)
						if err != nil {
							recordGlobalErr(err, LogEventSeenURLCheckFailed, &nt.URL)
							continue
						}
						if seen {
							continue
						}

						if nt.Type == scraper.TypeBook {
							exists, err := r.Redis.IsBookURLKnown(ctx, nt.URL)
							if err != nil {
								recordGlobalErr(err, LogEventBookExistsCheckFailed, &nt.URL)
								continue
							}
							if exists {
								totalProcessed.Add(1)
								continue
							}
						}

						activeTasks.Add(1)
						tasksChan <- nt
					}
				}(next)

				activeTasks.Done()
			}
		})
	}

	activeTasks.Add(1)
	tasksChan <- rootTask

	go func() {
		activeTasks.Wait()
		close(tasksChan)

		scraperWg.Wait()
		close(publishChan)
		close(imagesChan)
	}()

	wg.Wait()
	finalTotal := totalProcessed.Load()
	cancelReport()

	if cancelled.Load() {
		if err := r.Redis.PublishCancelled(ctx, crawlID, finalTotal); err != nil {
			return fmt.Errorf("failed to publish cancelled event: %w", err)
		}
		if err := r.Redis.ClearSeenURLs(ctx, crawlID); err != nil {
			slog.Error(string(LogEventSeenURLClearFailed), "crawl_id", crawlID, "error", err)
		}
		return nil
	}

	if err := r.Redis.PublishCompleted(ctx, crawlID, finalTotal); err != nil {
		return fmt.Errorf("failed to publish completed event: %w", err)
	}

	if err := r.Redis.ClearSeenURLs(ctx, crawlID); err != nil {
		slog.Error(string(LogEventSeenURLClearFailed), "crawl_id", crawlID, "error", err)
	}

	slog.Debug(
		string(LogEventCrawlCompleted),
		"source", source,
		"crawl_id", crawlID,
		"books_found", finalTotal,
		"duration", time.Since(start).String(),
	)

	return nil
}

func sourceTasks(s *scraper.Scraper) map[scraper.SourceName]scraper.Task {
	return map[scraper.SourceName]scraper.Task{
		scraper.SourceKnigaLv: {
			URL:     "https://kniga.lv/shop",
			Type:    scraper.TypeDiscovery,
			Handler: s.KnigaListingHandler,
		},
		scraper.SourceMnogoknig: {
			URL:     "https://mnogoknig.com/ru/categories/1/knigi",
			Type:    scraper.TypeDiscovery,
			Handler: s.MnogoknigCategoryHandler,
		},
	}
}
