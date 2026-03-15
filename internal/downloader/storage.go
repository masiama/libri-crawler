package downloader

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Storage interface {
	Save(ctx context.Context, key string, data io.Reader) error
	Exists(ctx context.Context, key string) bool
}

func NewStorage() Storage {
	if os.Getenv("STORAGE_TYPE") == "s3" {
		return connectBucket()
	}

	dir := os.Getenv("IMAGE_DIR")
	if dir == "" {
		dir = "./images"
	}
	os.MkdirAll(dir, os.ModePerm)
	return &LocalStorage{RootDir: dir}
}

type LocalStorage struct {
	RootDir string
}

func (l *LocalStorage) getImagePath(key string) string {
	return filepath.Join(l.RootDir, key)
}

func (l *LocalStorage) Save(ctx context.Context, key string, data io.Reader) error {
	path := l.getImagePath(key)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, data)
	return err
}

func (l *LocalStorage) Exists(ctx context.Context, key string) bool {
	_, err := os.Stat(l.getImagePath(key))
	return err == nil
}

type S3Service struct {
	client     *s3.Client
	bucketName string
}

func (s *S3Service) Save(ctx context.Context, key string, data io.Reader) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String("image/jpeg"),
	}
	_, err := s.client.PutObject(ctx, input)
	return err
}

func (s *S3Service) Exists(ctx context.Context, key string) bool {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	return err == nil
}

func connectBucket() *S3Service {
	bucketName := os.Getenv("CF_BUCKET_NAME")
	accountId := os.Getenv("CF_ACCOUNT_ID")
	accessKeyId := os.Getenv("CF_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("CF_ACCESS_KEY_SECRET")
	if bucketName == "" || accountId == "" || accessKeyId == "" || accessKeySecret == "" {
		log.Fatal("Cloudflare R2 credentials are not set in environment")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		log.Fatal(err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId))
	})

	return &S3Service{client: client, bucketName: bucketName}
}
