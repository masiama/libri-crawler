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
)

type LocalStorage struct {
	RootDir string
}

func NewStorage() (*LocalStorage, error) {
	dir := os.Getenv("IMAGES_DIR")
	if dir == "" {
		return nil, fmt.Errorf("IMAGES_DIR is not set")
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create directory %s: %w", dir, err)
	}
	return &LocalStorage{RootDir: dir}, nil
}

func (l *LocalStorage) Save(ctx context.Context, book scraper.ScrapedBook, data io.Reader) error {
	dir, fullPath := l.getShardedPath(book)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
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
