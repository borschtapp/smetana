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

	"borscht.app/smetana/domain"
	"github.com/doyensec/safeurl"
	_ "golang.org/x/image/webp"

	"borscht.app/smetana/internal/storage"
	"borscht.app/smetana/internal/utils"
)

type ImageService struct {
	storage storage.FileStorage
}

func NewImageService(s storage.FileStorage) *ImageService {
	return &ImageService{storage: s}
}

func (s *ImageService) DownloadAndSaveImage(imageURL string, savePath string) (*domain.UploadedImage, error) {
	// safeurl validates the resolved IP at connection time, preventing SSRF and DNS rebinding attacks
	client := safeurl.Client(safeurl.GetConfigBuilder().Build())
	resp, err := client.Get(imageURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
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

	return s.SaveImageData(savePath, data, contentType)
}

func (s *ImageService) SaveImageData(basePath string, data []byte, contentType string) (*domain.UploadedImage, error) {
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

	return &domain.UploadedImage{
		Path:   storage.Path(fullPath),
		Width:  width,
		Height: height,
	}, nil
}
