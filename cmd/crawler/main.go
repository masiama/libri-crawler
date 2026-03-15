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

	libriDB "libri/internal/db"
	libriDownloader "libri/internal/downloader"
	libriScraper "libri/internal/scraper"
)

const (
	scraperWorkers    = 3
	downloaderWorkers = 20
)

func main() {
	start := time.Now()
	loadEnv()

	dbClient := libriDB.ConnectDB()
	defer dbClient.Close()

	httpClient := &http.Client{Timeout: 15 * time.Second}
	store := libriDownloader.NewStorage()
	scraper := &libriScraper.Scraper{Client: httpClient, DB: dbClient}
	dl := &libriDownloader.Downloader{Store: store, Client: httpClient}

	var wg sync.WaitGroup
	var activeTasks sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tasksChan := make(chan libriScraper.Task, 10)
	saveChan := make(chan libriScraper.ScrapedBook, 500)
	imagesChan := make(chan libriScraper.ScrapedBook, 1000)

	wg.Go(func() {
		for book := range saveChan {
			scraper.SaveBook(ctx, book)
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
				node, err := scraper.Fetch(ctx, t.URL)
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

				go func(tasksToAdd []libriScraper.Task) {
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
	tasksChan <- libriScraper.Task{
		URL:     "https://kniga.lv/shop",
		Handler: scraper.KnigaListingHandler,
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
