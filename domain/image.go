package domain

import (
	"context"

	"borscht.app/smetana/internal/storage"
)

type UploadedImage struct {
	Path   storage.Path
	Width  int
	Height int
}

type ImageService interface {
	DownloadAndSaveImage(ctx context.Context, imageURL string, savePath string) (*UploadedImage, error)
	SaveImageData(basePath string, data []byte, contentType string) (*UploadedImage, error)
	DeleteImage(path storage.Path) error
}
