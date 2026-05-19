package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"libri-crawler/internal/downloader"
	"libri-crawler/internal/redis"
)

const (
	apiTimeout  = 60 * time.Second
	defaultPort = "8081"
)

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func main() {
	level := resolveLogLevel(os.Getenv("LOG_LEVEL"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Debug(string(LogEventCrawlerInitialized), "log_level", level.String())

	if err := loadEnv(); err != nil {
		fatal(LogEventRequiredEnvMissing, err)
	}

	httpClient := &http.Client{Timeout: apiTimeout}
	store, err := downloader.NewStorage()
	if err != nil {
		fatal(LogEventStorageInitializationFailed, err)
	}

	rdb, err := redis.NewFromEnv()
	if err != nil {
		fatal(LogEventRedisInitializationFailed, err)
	}
	if err := rdb.Ping(context.Background()); err != nil {
		fatal(LogEventRedisInitializationFailed, err)
	}
	defer rdb.Close()

	runner := &Runner{
		HTTPClient: httpClient,
		Store:      store,
		Redis:      rdb,
	}

	manager := NewCrawlManager(runner)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: newServer(manager),
	}

	slog.Info(string(LogEventCrawlerServerStarted), "port", port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(LogEventCrawlerServerFailed, err)
	}
}

func resolveLogLevel(raw string) slog.Level {
	level, ok := levelMap[raw]
	if !ok {
		return slog.LevelInfo
	}
	return level
}

func loadEnv() error {
	_ = godotenv.Load()

	for _, v := range []string{"REDIS_ADDR", "IMAGES_DIR"} {
		if os.Getenv(v) == "" {
			return fmt.Errorf("%s is not set", v)
		}
	}

	return nil
}

func fatal(event LogEvent, err error, attrs ...any) {
	slog.Error(string(event), append(attrs, "error", err)...)
	os.Exit(1)
}
