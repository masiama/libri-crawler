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
	scraperWorkers    = 25
	downloaderWorkers = 100
	saverWorkers      = 10
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

	slog.Info("logger initialized", "level", level.String())

	var scrapersRunning int32 = scraperWorkers
	var totalProcessed int64

	start := time.Now()
	loadEnv()

	httpClient := &http.Client{Timeout: 15 * time.Second}
	store, err := downloader.NewStorage()
	if err != nil {
		fatal("failed to initialize storage", err)
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
	var activeTasks sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tasksChan := make(chan scraper.Task, 10_000)
	saveChan := make(chan scraper.ScrapedBook, 20_000)
	imagesChan := make(chan scraper.ScrapedBook, 50_000)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			count := atomic.LoadInt64(&totalProcessed)
			slog.Debug("progress update", "books_processed", count)
		}
	}()

	for range saverWorkers {
		wg.Go(func() {
			var batch []scraper.ScrapedBook
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for {
				select {
				case b, ok := <-saveChan:
					if !ok {
						if len(batch) > 0 {
							s.SaveBatch(ctx, batch)
						}
						return
					}
					batch = append(batch, b)
					if len(batch) >= 100 {
						s.SaveBatch(ctx, batch)
						batch = nil
					}
				case <-ticker.C:
					if len(batch) > 0 {
						s.SaveBatch(ctx, batch)
						batch = nil
					}
				case <-ctx.Done():
					if len(batch) > 0 {
						s.SaveBatch(context.Background(), batch)
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
		wg.Go(func() {
			defer func() {
				if atomic.AddInt32(&scrapersRunning, -1) == 0 {
					close(saveChan)
					close(imagesChan)
				}
			}()

			for t := range tasksChan {
				node, err := s.Fetch(ctx, t.URL)
				if err != nil {
					slog.Error("failed to fetch URL", "url", t.URL, "error", err)
					activeTasks.Done()
					continue
				}

				next, books, _ := t.Handler(ctx, node)

				atomic.AddInt64(&totalProcessed, int64(len(books)))

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
								slog.Error("failed to check if book exists", "url", nt.URL, "error", err)
								continue
							}
							if exists {
								atomic.AddInt64(&totalProcessed, 1)
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
		slog.Info("starting scraper for all sources", "available_sources", scraper.GetSources())
		for _, t := range sources {
			activeTasks.Add(1)
			tasksChan <- t
		}
	default:
		sourceName := scraper.SourceName(*source)
		t, ok := sources[sourceName]
		if !ok {
			slog.Error("invalid source selected",
				"requested", *source,
				"valid_options", scraper.GetSources(),
			)
			return
		}
		slog.Info("starting scraper for single source", "source", *source)
		activeTasks.Add(1)
		tasksChan <- t
	}

	go func() {
		activeTasks.Wait()
		close(tasksChan)
	}()

	wg.Wait()
	slog.Info("scraping completed", "total_books_processed", totalProcessed, "duration", time.Since(start).String())
}

func loadEnv() {
	_ = godotenv.Load()

	for _, v := range []string{"API_URL", "INTERNAL_API_KEY", "IMAGES_DIR"} {
		if os.Getenv(v) == "" {
			fatal("environment variable is required", fmt.Errorf("%s is not set", v), "variable", v)
		}
	}
}

func fatal(msg string, err error, attrs ...any) {
	slog.Error(msg, append(attrs, "error", err)...)
	os.Exit(1)
}
