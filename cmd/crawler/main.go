package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"

	"libri-crawler/internal/db"
	"libri-crawler/internal/downloader"
	"libri-crawler/internal/scraper"
)

const (
	scraperWorkers    = 25
	downloaderWorkers = 100
	saverWorkers      = 10
)

func main() {
	var scrapersRunning int32 = scraperWorkers
	var totalProcessed int64

	start := time.Now()
	loadEnv()

	dbPool := db.ConnectDB()
	defer dbPool.Close()

	httpClient := &http.Client{Timeout: 15 * time.Second}
	store := downloader.NewStorage()
	cache := &scraper.URLCache{Items: make(map[string]struct{}, 100000)}

	s := &scraper.Scraper{Client: httpClient, DB: dbPool, Cache: cache}
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
			fmt.Printf("[%s] Status: %d books processed\n",
				time.Now().Format("15:04:05"), count)
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
					fmt.Printf("Failed to fetch %s: %v\n", t.URL, err)
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

						if nt.Type == scraper.TypeBook && s.BookExists(ctx, nt.URL) {
							atomic.AddInt64(&totalProcessed, 1)
							continue
						}

						activeTasks.Add(1)
						tasksChan <- nt
					}
				}(next)

				activeTasks.Done()
			}
		})
	}

	sources := map[string]scraper.Task{
		"kniga.lv": {
			URL:     "https://kniga.lv/shop",
			Type:    scraper.TypeDiscovery,
			Handler: s.KnigaListingHandler,
		},
		"mnogoknig.com": {
			URL:     "https://mnogoknig.com/ru/categories/1/knigi",
			Type:    scraper.TypeDiscovery,
			Handler: s.MnogoknigCategoryHandler,
		},
	}
	sourcesStr := ""
	for k := range sources {
		sourcesStr += fmt.Sprintf("'%s', ", k)
	}
	sourcesStr = sourcesStr[:len(sourcesStr)-2]

	source := flag.String("source", "all", fmt.Sprintf("Source to scrape: %s, or 'all'", sourcesStr))
	flag.Parse()

	switch *source {
	case "all":
		log.Printf("Starting scraper for all sources: %s\n", sourcesStr)
		for _, t := range sources {
			activeTasks.Add(1)
			tasksChan <- t
		}
	default:
		t, ok := sources[*source]
		if !ok {
			log.Fatalf("Invalid source '%s'. Valid options are: %s, or 'all'", *source, sourcesStr)
		}
		log.Printf("Starting scraper for source: %s\n", *source)
		activeTasks.Add(1)
		tasksChan <- t
	}

	go func() {
		activeTasks.Wait()
		close(tasksChan)
	}()

	wg.Wait()
	fmt.Printf("Scraping complete. Total time: %s\n", time.Since(start))
}

func loadEnv() {
	_ = godotenv.Load()

	criticalVars := []string{"DATABASE_URL"}

	if os.Getenv("STORAGE_TYPE") == "s3" {
		criticalVars = append(criticalVars, "CF_BUCKET_NAME", "CF_ACCOUNT_ID", "CF_ACCESS_KEY_ID", "CF_ACCESS_KEY_SECRET")
	}

	for _, v := range criticalVars {
		if os.Getenv(v) == "" {
			log.Fatalf("Environment variable %s is required", v)
		}
	}
}
