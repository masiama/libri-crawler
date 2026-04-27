package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"

	"libri-crawler/internal/api"
	"libri-crawler/internal/downloader"
	"libri-crawler/internal/scraper"
)

const (
	scraperWorkers    = 10
	downloaderWorkers = 100
	saverWorkers      = 3
	apiTimeout        = 60 * time.Second
)

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func getLevelsString() string {
	var levels []string
	for l := range levelMap {
		levels = append(levels, l)
	}
	return strings.Join(levels, ", ")
}

func main() {
	sourcesStr := scraper.GetSourcesString()
	source := flag.String("source", "all", fmt.Sprintf("Source to scrape: %s, or 'all'", sourcesStr))
	logLvl := flag.String("level", "info", fmt.Sprintf("Log level: %s", getLevelsString()))

	flag.Parse()

	level, ok := levelMap[strings.ToLower(*logLvl)]
	if !ok {
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info(string(LogEventCrawlerInitialized), "log_level", level.String())

	var totalProcessed atomic.Int64

	start := time.Now()
	loadEnv()

	httpClient := &http.Client{Timeout: apiTimeout}
	store, err := downloader.NewStorage()
	if err != nil {
		fatal(LogEventStorageInitializationFailed, err)
	}
	cache := scraper.NewURLCache(100_000)

	apiClient := &api.APIClient{
		BaseURL:    os.Getenv("API_URL"),
		APIKey:     os.Getenv("INTERNAL_API_KEY"),
		HTTPClient: httpClient,
	}
	s := &scraper.Scraper{Client: httpClient, Cache: cache, API: apiClient}
	dl := &downloader.Downloader{Store: store, Client: httpClient}

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tasksChan := make(chan scraper.Task, 10_000)
	saveChan := make(chan scraper.ScrapedBook, 20_000)
	imagesChan := make(chan scraper.ScrapedBook, 50_000)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var lastReported int64 = -1
		for range ticker.C {
			count := totalProcessed.Load()
			if count == lastReported {
				continue
			}
			lastReported = count
			slog.Info(string(LogEventCrawlProgress), "books_found", count)
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
				dl.Download(ctx, b)
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
					slog.Error(string(LogEventSourceFetchFailed), "url", t.URL, "error", err)
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
								slog.Error(string(LogEventBookExistsCheckFailed), "url", nt.URL, "error", err)
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

	sources := map[scraper.SourceName]scraper.Task{
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

	switch *source {
	case "all":
		slog.Info(string(LogEventCrawlStarted), "mode", "all", "sources", scraper.GetSources())
		for _, t := range sources {
			activeTasks.Add(1)
			tasksChan <- t
		}
	default:
		sourceName := scraper.SourceName(*source)
		t, ok := sources[sourceName]
		if !ok {
			slog.Error(string(LogEventInvalidSource), "requested_source", *source, "valid_sources", scraper.GetSources())
			return
		}
		slog.Info(string(LogEventCrawlStarted), "mode", "single", "source", *source)
		activeTasks.Add(1)
		tasksChan <- t
	}

	go func() {
		activeTasks.Wait()
		close(tasksChan)

		scraperWg.Wait()
		close(saveChan)
		close(imagesChan)
	}()

	wg.Wait()
	if saveErr != nil {
		fatal(LogEventBatchSaveFailed, saveErr)
	}
	slog.Info(string(LogEventCrawlCompleted), "books_found", totalProcessed.Load(), "duration", time.Since(start).String())
}

func loadEnv() {
	_ = godotenv.Load()

	for _, v := range []string{"API_URL", "INTERNAL_API_KEY", "IMAGES_DIR"} {
		if os.Getenv(v) == "" {
			fatal(LogEventRequiredEnvMissing, fmt.Errorf("%s is not set", v), "variable", v)
		}
	}
}

func fatal(event LogEvent, err error, attrs ...any) {
	slog.Error(string(event), append(attrs, "error", err)...)
	os.Exit(1)
}
