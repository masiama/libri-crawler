package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"libri-crawler/internal/redis"
	"libri-crawler/internal/scraper"
)

type crawlManager struct {
	runner *Runner
	rdb    *redis.Client
}

func NewCrawlManager(runner *Runner, rdb *redis.Client) *crawlManager {
	return &crawlManager{runner: runner, rdb: rdb}
}

func (m *crawlManager) ProcessCommand(ctx context.Context, crawlID int64, source scraper.SourceName) {
	if !scraper.IsValidSource(source) {
		slog.Error(string(LogEventInvalidSource), "crawlId", crawlID, "source", source)
		_ = m.rdb.PublishError(ctx, crawlID, fmt.Errorf("invalid source, expected one of: %s", scraper.GetSourcesString()))
		return
	}

	lockAcquired, err := m.rdb.AcquireCrawlLock(ctx, source, 10*time.Minute)
	if err != nil {
		slog.Error(string(LogEventLockAcquisitionFailed), "source", source, "error", err)
		return
	}
	if !lockAcquired {
		slog.Error(string(LogEventCrawlRejectedDuplicate), "source", source, "crawlId", crawlID)
		_ = m.rdb.PublishError(ctx, crawlID, fmt.Errorf("source %s is already running", source))
		return
	}

	go m.executeJob(source, crawlID)
}

func (m *crawlManager) executeJob(source scraper.SourceName, crawlID int64) {
	ctx := context.Background()

	cancelCtx, cancelWatchdog := context.WithCancel(ctx)
	defer cancelWatchdog()

	go func() {
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := m.rdb.ExtendCrawlLock(ctx, source, 10*time.Minute); err != nil {
					slog.Error(string(LogEventLockExtensionFailed), "source", source, "error", err)
				}
				if err := m.rdb.ExtendSeenURLs(ctx, crawlID, 10*time.Minute); err != nil {
					slog.Error(string(LogEventSeenURLExtensionFailed), "crawl_id", crawlID, "error", err)
				}
			case <-cancelCtx.Done():
				return
			}
		}
	}()

	defer func() {
		if err := m.rdb.ReleaseCrawlLock(ctx, source); err != nil {
			slog.Error(string(LogEventLockReleaseFailed), "source", source, "error", err)
		}
	}()

	if err := m.runner.Run(ctx, source, crawlID); err != nil {
		slog.Error(string(LogEventCrawlFailed), "source", source, "crawlId", crawlID, "error", err)
		_ = m.rdb.PublishError(ctx, crawlID, err)
	}
}
