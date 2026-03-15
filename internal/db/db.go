package db

import (
	"libri-crawler/ent"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func ConnectDB() *ent.Client {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set in environment")
	}

	client, err := ent.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed opening connection to postgres: %v", err)
	}

	return client
}
