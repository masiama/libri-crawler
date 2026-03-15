package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"

	"libri-crawler/internal/db"
	"libri-crawler/internal/downloader"
	"libri-crawler/internal/scraper"
)

const (
	scraperWorkers    = 3
	downloaderWorkers = 20
)

func main() {
	start := time.Now()
	loadEnv()

	dbClient := db.ConnectDB()
	defer dbClient.Close()

	httpClient := &http.Client{Timeout: 15 * time.Second}
	store := downloader.NewStorage()
	s := &scraper.Scraper{Client: httpClient, DB: dbClient}
	dl := &downloader.Downloader{Store: store, Client: httpClient}

	var wg sync.WaitGroup
	var activeTasks sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tasksChan := make(chan scraper.Task, 10)
	saveChan := make(chan scraper.ScrapedBook, 500)
	imagesChan := make(chan scraper.ScrapedBook, 1000)

	wg.Go(func() {
		for book := range saveChan {
			s.SaveBook(ctx, book)
		}
	})

	for range downloaderWorkers {
		wg.Go(func() {
			for b := range imagesChan {
				dl.Download(ctx, b)
			}
		})
	}

	for range scraperWorkers {
		wg.Go(func() {
			for t := range tasksChan {
				node, err := s.Fetch(ctx, t.URL)
				if err != nil {
					continue
				}

				next, books, _ := t.Handler(ctx, node)

				if len(books) > 0 {
					for _, b := range books {
						saveChan <- b
						imagesChan <- b
					}
				}

				go func(tasksToAdd []scraper.Task) {
					for _, nt := range tasksToAdd {
						activeTasks.Add(1)
						tasksChan <- nt
					}

					activeTasks.Done()
				}(next)
			}
		})
	}

	activeTasks.Add(1)
	tasksChan <- scraper.Task{
		URL:     "https://kniga.lv/shop",
		Handler: s.KnigaListingHandler,
	}

	go func() {
		activeTasks.Wait()
		close(tasksChan)
		close(saveChan)
		close(imagesChan)
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
