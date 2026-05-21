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
	apiTimeout          = 60 * time.Second
	maxIdleConns        = 100
	maxIdleConnsPerHost = 10
	idleConnTimeout     = 90 * time.Second
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

	transport := &http.Transport{MaxIdleConns: maxIdleConns, MaxIdleConnsPerHost: maxIdleConnsPerHost, IdleConnTimeout: idleConnTimeout}
	httpClient := &http.Client{Timeout: apiTimeout, Transport: transport}
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
	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Error(string(LogEventRedisCloseFailed), "error", err)
		}
	}()

	runner := &Runner{
		HTTPClient: httpClient,
		Store:      store,
		Redis:      rdb,
	}

	manager := NewCrawlManager(runner, rdb)
	ctx := context.Background()

	slog.Info(string(LogEventCrawlerDaemonStarted))

	for {
		cmd, err := rdb.ListenForCommands(ctx)
		if err != nil {
			slog.Error(string(LogEventRedisCommandFetchFailed), "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		slog.Debug(string(LogEventCommandReceived), "crawlId", cmd.CrawlID, "source", cmd.Source)

		manager.ProcessCommand(ctx, cmd.CrawlID, cmd.Source)
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

	for _, v := range []string{"REDIS_URL", "IMAGES_DIR"} {
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
