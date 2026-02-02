package services

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"

	_ "golang.org/x/image/webp"

	"borscht.app/smetana/pkg/storage"
	"borscht.app/smetana/pkg/utils"
)

type ImageService struct {
	storage storage.FileStorage
}

func NewImageService(s storage.FileStorage) *ImageService {
	return &ImageService{storage: s}
}

type UploadedImage struct {
	Path   storage.Path
	Width  int
	Height int
}

func (s *ImageService) DownloadAndPutImage(imageUrl string, savePath string) (*UploadedImage, error) {
	if !utils.IsPublicURL(imageUrl) {
		return nil, errors.New("invalid or non-public image URL")
	}

	resp, err := http.Get(imageUrl) // #nosec G107
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		return nil, errors.New("unable to download image")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("unable to read response body")
	}

	contentType := utils.DetectContentTypeFromResponse(resp)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}

	return s.SaveImage(savePath, data, contentType)
}

func (s *ImageService) SaveImage(basePath string, data []byte, contentType string) (*UploadedImage, error) {
	fullPath := basePath
	if filepath.Ext(basePath) == "" {
		extension := utils.ExtensionByType(contentType)
		if extension == "" {
			extension = ".jpg"
		}
		fullPath += extension
	}

	if err := s.storage.Save(fullPath, bytes.NewBuffer(data), int64(len(data)), contentType); err != nil {
		return nil, err
	}

	width, height := 0, 0
	if config, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		width = config.Width
		height = config.Height
	}

	return &UploadedImage{
		Path:   storage.Path(fullPath),
		Width:  width,
		Height: height,
	}, nil
}
