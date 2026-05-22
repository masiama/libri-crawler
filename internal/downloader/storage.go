package downloader

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"libri-crawler/internal/scraper"
	"os"
	"path/filepath"
	"sync"
)

type LocalStorage struct {
	RootDir  string
	mu       sync.RWMutex
	dirCache map[string]struct{}
}

func NewStorage() (*LocalStorage, error) {
	dir := os.Getenv("IMAGES_DIR")
	if dir == "" {
		return nil, fmt.Errorf("IMAGES_DIR is not set")
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create directory %s: %w", dir, err)
	}
	return &LocalStorage{RootDir: dir, dirCache: make(map[string]struct{})}, nil
}

func (l *LocalStorage) Save(ctx context.Context, book scraper.ScrapedBook, data io.Reader) error {
	dir, fullPath := l.getShardedPath(book)

	if err := l.maybeCreateDir(dir); err != nil {
		return err
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(f, data)
	return err
}

func (l *LocalStorage) Exists(ctx context.Context, book scraper.ScrapedBook) bool {
	_, path := l.getShardedPath(book)
	_, err := os.Stat(path)
	return err == nil
}

func (l *LocalStorage) getShardedPath(book scraper.ScrapedBook) (string, string) {
	sum := md5.Sum([]byte(book.ISBN))

	shard1 := hex.EncodeToString(sum[0:1])
	shard2 := hex.EncodeToString(sum[1:2])

	dir := filepath.Join(l.RootDir, shard1, shard2)
	fullPath := filepath.Join(dir, book.ISBN+".jpg")
	return dir, fullPath
}

func (l *LocalStorage) maybeCreateDir(dir string) error {
	l.mu.RLock()
	_, warmed := l.dirCache[dir]
	l.mu.RUnlock()

	if warmed {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, warmed = l.dirCache[dir]; warmed {
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	l.dirCache[dir] = struct{}{}
	return nil
}
