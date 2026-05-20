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

type Config struct {
	RedisAddr string
	ImagesDir string
	LogLevel  string
}

func main() {
	cfg, err := loadEnv()
	if err != nil {
		fmt.Printf("Startup error: %v\n", err)
		os.Exit(1)
	}

	level := resolveLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Debug(string(LogEventCrawlerInitialized), "log_level", level.String())

	transport := &http.Transport{MaxIdleConns: maxIdleConns, MaxIdleConnsPerHost: maxIdleConnsPerHost, IdleConnTimeout: idleConnTimeout}
	httpClient := &http.Client{Timeout: apiTimeout, Transport: transport}
	store, err := downloader.NewStorage(cfg.ImagesDir) 
	if err != nil {
		fatal(LogEventStorageInitializationFailed, err)
	}

	rdb, err := redis.NewClient(cfg.RedisAddr) 
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

func loadEnv() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		RedisAddr: os.Getenv("REDIS_ADDR"),
		ImagesDir: os.Getenv("IMAGES_DIR"),
		LogLevel:  os.Getenv("LOG_LEVEL"),
	}

	if cfg.RedisAddr == "" {
		return cfg, fmt.Errorf("REDIS_ADDR is not set")
	}
	if cfg.ImagesDir == "" {
		return cfg, fmt.Errorf("IMAGES_DIR is not set")
	}

	return cfg, nil
}

func fatal(event LogEvent, err error, attrs ...any) {
	slog.Error(string(event), append(attrs, "error", err)...)
	os.Exit(1)
}
