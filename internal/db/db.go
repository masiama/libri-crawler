package db

import (
	"database/sql"
	"libri-crawler/ent"
	"log"
	"os"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
)

func ConnectDB() *ent.Client {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set in environment")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}

	drv := entsql.OpenDB(dialect.Postgres, db)

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	return ent.NewClient(ent.Driver(drv))
}
