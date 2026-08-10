package services_test

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/storage"
)

// --- fixtures ---

// validPNG and validJPEG are real, fully-decodable images (not just magic-byte headers) so
// tests exercise the same image.DecodeConfig check the service performs, not just sniffing.

func validPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))))
	return buf.Bytes()
}

func validJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil))
	return buf.Bytes()
}

// --- fakes ---

type fakeFileStorage struct {
	saved   map[string][]byte
	deleted []string
	baseURL string
	saveErr error
}

func newFakeStorage() *fakeFileStorage {
	return &fakeFileStorage{saved: make(map[string][]byte), baseURL: "http://cdn.test"}
}

func (f *fakeFileStorage) Save(path string, _ io.Reader, _ int64, _ string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved[path] = nil
	return nil
}
func (f *fakeFileStorage) Delete(path string) error { f.deleted = append(f.deleted, path); return nil }
func (f *fakeFileStorage) GetBaseURL() string       { return f.baseURL }

// fakeFileStorage must satisfy storage.FileStorage — io.Reader parameter requires the full interface.
var _ storage.FileStorage = (*fakeFileStorage)(nil)

type fakeImageRepo struct {
	images          map[uuid.UUID]*domain.Image
	bySourceURL     map[string]*domain.Image
	findBySourceErr error
	createErr       error
}

func newFakeImageRepo() *fakeImageRepo {
	return &fakeImageRepo{
		images:      make(map[uuid.UUID]*domain.Image),
		bySourceURL: make(map[string]*domain.Image),
	}
}

func (r *fakeImageRepo) Create(image *domain.Image) error {
	if r.createErr != nil {
		return r.createErr
	}
	// Simulate BeforeCreate hook: assign ID if not set.
	if image.ID == uuid.Nil {
		image.ID, _ = uuid.NewV7()
	}
	r.images[image.ID] = image
	if image.SourceURL != "" {
		r.bySourceURL[image.SourceURL] = image
	}
	return nil
}

func (r *fakeImageRepo) Update(image *domain.Image) error {
	if _, ok := r.images[image.ID]; !ok {
		return sentinels.ErrNotFound
	}
	r.images[image.ID] = image
	return nil
}

func (r *fakeImageRepo) FindByID(id uuid.UUID) (*domain.Image, error) {
	image, ok := r.images[id]
	if !ok {
		return nil, sentinels.ErrNotFound
	}
	return image, nil
}

func (r *fakeImageRepo) FindByEntity(entityType string, entityID uuid.UUID) ([]*domain.Image, error) {
	var out []*domain.Image
	for _, image := range r.images {
		if image.EntityType == entityType && image.EntityID == entityID {
			out = append(out, image)
		}
	}
	return out, nil
}

func (r *fakeImageRepo) FindDefault(entityType string, entityID uuid.UUID) (*domain.Image, error) {
	for _, image := range r.images {
		if image.EntityType == entityType && image.EntityID == entityID && image.IsDefault {
			return image, nil
		}
	}
	return nil, sentinels.ErrNotFound
}

func (r *fakeImageRepo) FindBySourceURL(sourceURL string) (*domain.Image, error) {
	if r.findBySourceErr != nil {
		return nil, r.findBySourceErr
	}
	if image, ok := r.bySourceURL[sourceURL]; ok {
		return image, nil
	}
	return nil, sentinels.ErrNotFound
}

func (r *fakeImageRepo) SetDefault(target *domain.Image) error {
	for _, image := range r.images {
		if image.EntityType == target.EntityType && image.EntityID == target.EntityID {
			image.IsDefault = false
		}
	}
	if image, ok := r.images[target.ID]; ok {
		image.IsDefault = true
	}
	return nil
}

func (r *fakeImageRepo) Delete(id uuid.UUID) error {
	delete(r.images, id)
	return nil
}

func newImageService(t *testing.T) (domain.ImageService, *fakeFileStorage, *fakeImageRepo) {
	t.Helper()
	s := newFakeStorage()
	r := newFakeImageRepo()
	svc := services.NewImageService(s, r)
	return svc, s, r
}

// --- tests ---

func TestImageService_PersistUploaded_StoresFileAndReturnsAbsoluteURL(t *testing.T) {
	svc, fs, _ := newImageService(t)

	uploaded, err := svc.PersistUploaded(context.Background(), validJPEG(t), "image/jpeg")

	require.NoError(t, err)
	assert.True(t, uploaded.Path != "", "path must not be empty")
	assert.Contains(t, string(uploaded.Path), "uploads/")
	// Exactly one file must be saved.
	assert.Len(t, fs.saved, 1)
}

func TestImageService_PersistUploaded_DerivesExtensionFromContentType(t *testing.T) {
	svc, fs, _ := newImageService(t)

	_, err := svc.PersistUploaded(context.Background(), validPNG(t), "image/png")

	require.NoError(t, err)
	assert.Len(t, fs.saved, 1)
	for path := range fs.saved {
		assert.True(t, strings.HasSuffix(path, ".png"), "expected .png extension, got %s", path)
	}
}

func TestImageService_PersistUploaded_RejectsSpoofedContentType(t *testing.T) {
	svc, fs, _ := newImageService(t)

	// HTML bytes, but claiming to be an image — the caller-supplied contentType must never
	// be trusted, since it would otherwise let hostile content get stored and later served
	// as active content (e.g. under /uploads via the static file route).
	html := []byte("<html><body><script>alert(document.cookie)</script></body></html>")
	uploaded, err := svc.PersistUploaded(context.Background(), html, "image/jpeg")

	require.Error(t, err)
	assert.Nil(t, uploaded)
	assert.Empty(t, fs.saved, "hostile content must never reach storage")
}

func TestImageService_PersistUploaded_RejectsCorruptImageData(t *testing.T) {
	svc, fs, _ := newImageService(t)

	// Valid PNG magic bytes so it sniffs as image/png, but truncated so it cannot decode —
	// must be rejected rather than stored with fabricated (zero) dimensions.
	corrupt := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	uploaded, err := svc.PersistUploaded(context.Background(), corrupt, "image/png")

	require.Error(t, err)
	assert.Nil(t, uploaded)
	assert.Empty(t, fs.saved)
}

func TestImageService_PersistUploaded_RejectsDisallowedImageType(t *testing.T) {
	svc, fs, _ := newImageService(t)

	// BMP sniffs to a real, non-spoofed content type, but it's outside the allowlist.
	bmp := []byte{0x42, 0x4D, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	uploaded, err := svc.PersistUploaded(context.Background(), bmp, "image/bmp")

	require.Error(t, err)
	assert.Nil(t, uploaded)
	assert.Empty(t, fs.saved)
}

func TestImageService_SetDefault_MarksImageAsDefault(t *testing.T) {
	svc, _, repo := newImageService(t)

	entityID := uuid.New()
	imgID, _ := uuid.NewV7()
	repo.images[imgID] = &domain.Image{
		ID:         imgID,
		EntityType: "publishers",
		EntityID:   entityID,
		Path:       new(storage.Path("publisher/abc/img.jpg")),
	}

	err := svc.SetDefault(repo.images[imgID])

	require.NoError(t, err)
	assert.True(t, repo.images[imgID].IsDefault)
	// Path is unchanged — storage.Path.MarshalJSON resolves it to an absolute URL on the wire.
	assert.Equal(t, storage.Path("publisher/abc/img.jpg"), *repo.images[imgID].Path)
}

func TestImageService_Delete_RemovesStorageFileAndDBRecord(t *testing.T) {
	svc, fs, repo := newImageService(t)

	imgID, _ := uuid.NewV7()
	repo.images[imgID] = &domain.Image{
		ID:         imgID,
		EntityType: "food",
		EntityID:   uuid.New(),
		Path:       new(storage.Path("food/abc/img.jpg")),
	}

	require.NoError(t, svc.Delete(imgID))

	assert.Contains(t, fs.deleted, "food/abc/img.jpg")
	_, err := repo.FindByID(imgID)
	require.ErrorIs(t, err, sentinels.ErrNotFound)
}
