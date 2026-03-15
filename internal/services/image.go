package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"time"

	"github.com/doyensec/safeurl"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/storage"
	"borscht.app/smetana/internal/utils"
)

type imageService struct {
	repo       domain.ImageRepository
	httpClient *safeurl.WrappedClient
	userAgent  string
	storage    storage.FileStorage
	timeout    time.Duration
}

func NewImageService(s storage.FileStorage, repo domain.ImageRepository) domain.ImageService {
	timeout := utils.GetenvDuration("DOWNLOAD_TIMEOUT", 30*time.Second)
	client := safeurl.Client(safeurl.GetConfigBuilder().SetTimeout(timeout).Build())
	userAgent := utils.Getenv("USER_AGENT", "Mozilla/5.0 (Windows NT 10.0; rv:80.0) Gecko/20100101 Firefox/80.0")
	return &imageService{
		httpClient: client,
		userAgent:  userAgent,
		storage:    s,
		repo:       repo,
		timeout:    timeout,
	}
}

func (s *imageService) PersistRemote(ctx context.Context, img *domain.Image, pathPrefix string) error {
	if pathPrefix == "" {
		pathPrefix = img.EntityType + "/" + img.EntityID.String()
	}

	// Dedup: reuse a previously downloaded copy. Nil Path means a prior attempt failed — retry.
	if existing, err := s.repo.FindBySourceURL(img.SourceURL); err == nil {
		if existing.Path != nil {
			*img = *existing
			return nil
		}
		return s.fillFromRemote(ctx, existing, pathPrefix)
	}

	if err := s.repo.Create(img); err != nil {
		return fmt.Errorf("create image record: %w", err)
	}

	return s.fillFromRemote(ctx, img, pathPrefix)
}

// fillFromRemote downloads img.SourceURL, saves the file, and updates image with path + metadata.
// On failure the DB record is left intact (empty Path) so the caller can retry later.
func (s *imageService) fillFromRemote(ctx context.Context, img *domain.Image, pathPrefix string) error {
	data, contentType, err := s.download(ctx, img.SourceURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", img.SourceURL, err)
	}

	ext := utils.ExtensionByType(contentType)
	if ext == "" {
		ext = ".jpg"
	}
	fullPath := pathPrefix + "/" + img.ID.String() + ext

	if err := s.storage.Save(fullPath, bytes.NewBuffer(data), int64(len(data)), contentType); err != nil {
		return fmt.Errorf("save to storage: %w", err)
	}

	w, h := decodeImageDimensions(data)
	img.Path = new(storage.Path(fullPath))
	img.Width = w
	img.Height = h
	img.ContentType = contentType
	img.Size = int64(len(data))

	if err := s.repo.Update(img); err != nil {
		_ = s.storage.Delete(fullPath)
		return fmt.Errorf("update image path: %w", err)
	}

	return nil
}

func (s *imageService) PersistUploaded(_ context.Context, data []byte, contentType string) (*domain.UploadedImage, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	ext := utils.ExtensionByType(contentType)
	if ext == "" {
		ext = ".jpg"
	}
	fullPath := "uploads/" + id.String() + ext

	if err := s.storage.Save(fullPath, bytes.NewBuffer(data), int64(len(data)), contentType); err != nil {
		return nil, err
	}

	w, h := decodeImageDimensions(data)
	return &domain.UploadedImage{
		Path:        storage.Path(fullPath),
		Width:       w,
		Height:      h,
		ContentType: contentType,
		Size:        int64(len(data)),
	}, nil
}

func (s *imageService) SetDefault(image *domain.Image) error {
	return s.repo.SetDefault(image)
}

func (s *imageService) Delete(imageID uuid.UUID) error {
	img, err := s.repo.FindByID(imageID)
	if err != nil {
		return err
	}

	// Best-effort: log storage failure but still remove the DB record.
	if img.Path != nil {
		_ = s.storage.Delete(string(*img.Path))
	}

	return s.repo.Delete(imageID)
}

func (s *imageService) download(ctx context.Context, remoteURL string) ([]byte, string, error) {
	dlCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.New("unable to download image: non-200 status")
	}

	const maxImageBytes = 20 << 20 // 20 MB
	limited := io.LimitReader(resp.Body, maxImageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("read response body: %w", err)
	}
	if int64(len(data)) > maxImageBytes {
		return nil, "", errors.New("remote image too large (>20 MB)")
	}

	contentType := utils.DetectContentTypeFromHeader(resp.Header)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}

	return data, contentType, nil
}

func decodeImageDimensions(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}
