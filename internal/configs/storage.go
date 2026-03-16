package configs

import (
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3/log"
	fiberS3 "github.com/gofiber/storage/s3/v2"

	"borscht.app/smetana/internal/storage"
	"borscht.app/smetana/internal/utils"
)

type StorageConfig struct {
	Storage     storage.FileStorage
	BaseURL     string
	StorageRoot string // non-empty only for local storage; use to register a static file route
}

func NewStorage(serverHost string, serverPort int) StorageConfig {
	if os.Getenv("S3_BUCKET") != "" {
		s3Endpoint := utils.Getenv("S3_ENDPOINT", os.Getenv("S3_HOST"))
		if s3Endpoint != "" && !strings.Contains(s3Endpoint, "://") {
			s3Endpoint = "https://" + s3Endpoint
		}
		baseURL := fmt.Sprintf("%s/%s", s3Endpoint, os.Getenv("S3_BUCKET"))

		log.Infow("Cloud storage initialized", "baseURL", baseURL)
		return StorageConfig{
			Storage: storage.NewS3Storage(fiberS3.Config{
				Bucket:   os.Getenv("S3_BUCKET"),
				Endpoint: s3Endpoint,
				Region:   utils.Getenv("S3_REGION", "us-east-1"),
				Credentials: fiberS3.Credentials{
					AccessKey:       os.Getenv("S3_ACCESS_KEY"),
					SecretAccessKey: os.Getenv("S3_SECRET_KEY"),
				},
			}, baseURL),
			BaseURL: baseURL,
		}
	}

	baseURL := os.Getenv("BASE_URL")
	storageRoot := utils.Getenv("STORAGE_ROOT", "./data/uploads")
	if baseURL == "" {
		uploadsHost := serverHost
		if uploadsHost == "" {
			uploadsHost = "localhost"
		}
		baseURL = fmt.Sprintf("http://%s:%d/uploads", uploadsHost, serverPort)
	}

	log.Infow("Storage initialized", "baseURL", baseURL, "storageRoot", storageRoot)
	return StorageConfig{
		Storage:     storage.NewLocalStorage(storageRoot, baseURL),
		BaseURL:     baseURL,
		StorageRoot: storageRoot,
	}
}
