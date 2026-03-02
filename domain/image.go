package domain

import (
	"borscht.app/smetana/pkg/storage"
)

type UploadedImage struct {
	Path   storage.Path
	Width  int
	Height int
}

type ImageService interface {
	DownloadAndSaveImage(imageURL string, savePath string) (*UploadedImage, error)
	SaveImageData(basePath string, data []byte, contentType string) (*UploadedImage, error)
}
