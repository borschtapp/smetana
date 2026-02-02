package storage

import (
	"context"
	"io"
	"time"

	"borscht.app/smetana/pkg/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	fiberS3 "github.com/gofiber/storage/s3/v2"
)

type S3Storage struct {
	store   *fiberS3.Storage
	config  fiberS3.Config
	baseURL string
}

func NewS3Storage(config fiberS3.Config, baseUrl string) *S3Storage {
	store := fiberS3.New(config)
	return &S3Storage{
		store:   store,
		config:  config,
		baseURL: baseUrl,
	}
}

func (s *S3Storage) GetBaseURL() string {
	return s.baseURL
}

func (s *S3Storage) Save(path string, content io.Reader, size int64, contentType string) error {
	// The fiber/storage/s3 Set method takes []byte and doesn't allow streaming or Content-Type setting.
	// So we use the underlying connection.

	client := s.store.Conn()

	// We need to read content to bytes if we use simple PutObject, but since interface provides Reader,
	// we should try to use it directly.
	// However, aws-sdk-go-v2 PutObject Input takes io.Reader via Body.

	// Note: fiber/storage/s3 Conn() returns *s3.Client

	timeout := time.Duration(utils.GetenvInt("S3_TIMEOUT", 10)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Assuming we can read all into memory if size is small, or pass reader.
	// AWS SDK v2 allow Body as io.Reader.

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.config.Bucket),
		Key:           aws.String(path),
		Body:          content,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})

	return err
}

func (s *S3Storage) Delete(path string) error {
	return s.store.Delete(path)
}
