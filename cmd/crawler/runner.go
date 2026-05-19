package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"libri-crawler/internal/api"
	"libri-crawler/internal/downloader"
	"libri-crawler/internal/scraper"
)

const (
	scraperWorkers    = 10
	downloaderWorkers = 100
	saverWorkers      = 3
)

type Runner struct {
	HTTPClient *http.Client
	Store      *downloader.LocalStorage
	APIClient  *api.APIClient
}

func (r *Runner) Run(ctx context.Context, source scraper.SourceName) error {
	start := time.Now()
	slog.Info(string(LogEventCrawlStarted), "source", source)

	var totalProcessed atomic.Int64

	cache := scraper.NewURLCache(100_000)
	s := &scraper.Scraper{Client: r.HTTPClient, Cache: cache, API: r.APIClient}
	dl := &downloader.Downloader{Store: r.Store, Client: r.HTTPClient}

	rootTask, ok := sourceTasks(s)[source]
	if !ok {
		return ErrInvalidSource
	}

	var wg sync.WaitGroup
	var scraperWg sync.WaitGroup
	var activeTasks sync.WaitGroup
	var saveErrOnce sync.Once
	var saveErr error
	recordSaveErr := func(err error) {
		if err == nil {
			return
		}
		saveErrOnce.Do(func() {
			saveErr = err
		})
	}

	tasksChan := make(chan scraper.Task, 10_000)
	saveChan := make(chan scraper.ScrapedBook, 20_000)
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
				slog.Info(string(LogEventCrawlProgress), "source", source, "books_found", count)
			}
		}
	}()

	for range saverWorkers {
		wg.Go(func() {
			var batch []scraper.ScrapedBook
			ticker := time.NewTicker(time.Second * 5)
			defer ticker.Stop()

			for {
				select {
				case b, ok := <-saveChan:
					if !ok {
						if len(batch) > 0 {
							recordSaveErr(s.SaveBatch(ctx, batch))
						}
						return
					}
					batch = append(batch, b)
					if len(batch) >= 100 {
						recordSaveErr(s.SaveBatch(ctx, batch))
						batch = nil
					}
				case <-ticker.C:
					if len(batch) > 0 {
						recordSaveErr(s.SaveBatch(ctx, batch))
						batch = nil
					}
				case <-ctx.Done():
					if len(batch) > 0 {
						recordSaveErr(s.SaveBatch(context.Background(), batch))
					}
					return
				}
			}
		})
	}

	for range downloaderWorkers {
		wg.Go(func() {
			for b := range imagesChan {
				if err := dl.Download(ctx, b); err != nil {
					slog.Error(string(LogEventImageDownloadFailed), "source", source, "isbn", b.ISBN, "error", err)
				}
			}
		})
	}

	for range scraperWorkers {
		scraperWg.Add(1)
		wg.Go(func() {
			defer scraperWg.Done()

			for t := range tasksChan {
				node, err := s.Fetch(ctx, t.URL)
				if err != nil {
					slog.Error(string(LogEventSourceFetchFailed), "source", source, "url", t.URL, "error", err)
					activeTasks.Done()
					continue
				}

				next, books, _ := t.Handler(ctx, node)

				totalProcessed.Add(int64(len(books)))

				for _, b := range books {
					saveChan <- b
					imagesChan <- b
				}

				activeTasks.Add(1)
				go func(tasksToAdd []scraper.Task) {
					defer activeTasks.Done()
					for _, nt := range tasksToAdd {
						if s.Cache.Seen(nt.URL) {
							continue
						}

						if nt.Type == scraper.TypeBook {
							exists, err := s.BookExists(ctx, nt.URL)
							if err != nil {
								slog.Error(string(LogEventBookExistsCheckFailed), "source", source, "url", nt.URL, "error", err)
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
		close(saveChan)
		close(imagesChan)
	}()

	wg.Wait()

	cancelReport()

	if saveErr != nil {
		slog.Error(string(LogEventBatchSaveFailed), "source", source, "error", saveErr)
		return saveErr
	}

	slog.Info(
		string(LogEventCrawlCompleted),
		"source", source,
		"books_found", totalProcessed.Load(),
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
