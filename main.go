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
)

const (
	scraperWorkers    = 3
	downloaderWorkers = 20
)

func main() {
	start := time.Now()
	loadEnv()

	dbClient := connectDB()
	defer dbClient.Close()

	httpClient := &http.Client{Timeout: 15 * time.Second}
	store := NewStorage()
	scraper := &Scraper{Client: httpClient, DB: dbClient}
	dl := &Downloader{Store: store, Client: httpClient}

	var wg sync.WaitGroup
	var scraperWg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pagesChan := make(chan int, 10)
	imagesChan := make(chan localBook, 1000)

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

			for p := range pagesChan {
				books, stop, err := scraper.Scrape(ctx, p)

				if err != nil {
					fmt.Printf("Skipping Page %d: %v\n", p, err)
					continue
				}

				if stop {
					return
				}

				if len(books) > 0 {
					scraper.saveBooks(ctx, books)
					for _, b := range books {
						select {
						case imagesChan <- b:
						case <-ctx.Done():
							return
						}
					}
				}

				fmt.Printf("Page %d scraped. %d books found.\n", p, len(books))
			}
		})
	}

	go func() {
		scraperWg.Wait()
		close(imagesChan)
	}()

	for i := 1; ; i++ {
		select {
		case <-ctx.Done():
			close(pagesChan)
			wg.Wait()
			fmt.Printf("Scraping complete. Total time: %s\n", time.Since(start))
			return
		case pagesChan <- i:
			// Page sent
		}
	}
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
