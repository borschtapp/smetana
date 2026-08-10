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

func (s *imageService) PersistRemoteAsDefault(ctx context.Context, img *domain.Image, entityType string, entityID uuid.UUID, pathPrefix string) (*storage.Path, error) {
	if img == nil || img.SourceURL == "" {
		return nil, nil // exit gracefully, do not fail
	}
	if img.ID != uuid.Nil || img.Path != nil {
		return img.Path, nil // the image was saved before
	}

	img.EntityType = entityType
	img.EntityID = entityID
	if err := s.PersistRemote(ctx, img, pathPrefix); err != nil {
		return nil, fmt.Errorf("persist remote as default (persist): %w", err)
	}
	if err := s.SetDefault(img); err != nil {
		return nil, fmt.Errorf("persist remote as default (set default): %w", err)
	}
	return img.Path, nil
}

// fillFromRemote downloads img.SourceURL, saves the file, and updates image with path + metadata.
// On failure the DB record is left intact (empty Path) so the caller can retry later.
func (s *imageService) fillFromRemote(ctx context.Context, img *domain.Image, pathPrefix string) error {
	data, err := s.download(ctx, img.SourceURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", img.SourceURL, err)
	}

	contentType, ext, w, h, err := validateImage(data)
	if err != nil {
		return fmt.Errorf("validate remote image %s: %w", img.SourceURL, err)
	}
	fullPath := pathPrefix + "/" + img.ID.String() + ext

	if err := s.storage.Save(fullPath, bytes.NewBuffer(data), int64(len(data)), contentType); err != nil {
		return fmt.Errorf("save to storage: %w", err)
	}

	img.Path = new(storage.Path(fullPath))
	img.Width = &w
	img.Height = &h
	img.ContentType = &contentType
	img.Size = new(int64(len(data)))

	if err := s.repo.Update(img); err != nil {
		_ = s.storage.Delete(fullPath)
		return fmt.Errorf("update image path: %w", err)
	}

	return nil
}

// PersistUploaded ignores the caller-supplied contentType for storage decisions — it re-derives
// the type from the actual bytes so a caller that skips (or gets tricked past) its own validation
// can't smuggle in an unsafe extension or content type.
func (s *imageService) PersistUploaded(_ context.Context, data []byte, _ string) (*domain.UploadedImage, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("persist uploaded (generate uuid): %w", err)
	}

	contentType, ext, w, h, err := validateImage(data)
	if err != nil {
		return nil, fmt.Errorf("persist uploaded (validate): %w", err)
	}
	fullPath := "uploads/" + id.String() + ext

	if err := s.storage.Save(fullPath, bytes.NewBuffer(data), int64(len(data)), contentType); err != nil {
		return nil, fmt.Errorf("persist uploaded (save to storage): %w", err)
	}

	return &domain.UploadedImage{
		Path:        storage.Path(fullPath),
		Width:       w,
		Height:      h,
		ContentType: contentType,
		Size:        int64(len(data)),
	}, nil
}

func (s *imageService) SetDefault(image *domain.Image) error {
	if err := s.repo.SetDefault(image); err != nil {
		return fmt.Errorf("set default: %w", err)
	}
	return nil
}

func (s *imageService) Delete(imageID uuid.UUID) error {
	img, err := s.repo.FindByID(imageID)
	if err != nil {
		return fmt.Errorf("delete (fetch existing): %w", err)
	}

	// Best-effort: log storage failure but still remove the DB record.
	if img.Path != nil {
		_ = s.storage.Delete(string(*img.Path))
	}

	if err := s.repo.Delete(imageID); err != nil {
		return fmt.Errorf("delete (persist): %w", err)
	}
	return nil
}

// download fetches remoteURL and returns the raw response bytes. The response's declared
// Content-Type is intentionally not trusted or returned here — validateImage sniffs and
// verifies the real type from the bytes themselves.
func (s *imageService) download(ctx context.Context, remoteURL string) ([]byte, error) {
	dlCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute download request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unable to download image: non-200 status")
	}

	const maxImageBytes = 20 << 20 // 20 MB
	limited := io.LimitReader(resp.Body, maxImageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if int64(len(data)) > maxImageBytes {
		return nil, errors.New("remote image too large (>20 MB)")
	}

	return data, nil
}

// validateImage is the single gate remote downloads and uploads must both pass before bytes
// are written to storage or served back to clients:
//  1. sniff the real content type from the bytes (never trust a caller- or server-supplied header)
//  2. reject anything outside the image allowlist, so nothing (e.g. HTML) can ever be stored
//     with a content type or extension that a static file server would treat as active content
//  3. require the bytes to actually decode as an image, and as the *same* format that was
//     sniffed, to reject truncated files and polyglot payloads crafted to sniff as one format
//     while decoding as another
func validateImage(data []byte) (contentType, ext string, width, height int, err error) {
	contentType = http.DetectContentType(data)
	imgType, allowed := utils.AllowedImageTypes[contentType]
	if !allowed {
		return "", "", 0, 0, fmt.Errorf("unsupported image content type: %s", contentType)
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("invalid or corrupt image data: %w", err)
	}
	if imgType != format {
		return "", "", 0, 0, fmt.Errorf("image content mismatch: sniffed %s but decoded as %s", contentType, format)
	}

	return contentType, utils.ExtensionByType(contentType), cfg.Width, cfg.Height, nil
}
