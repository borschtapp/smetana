package services_test

import (
	"context"
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
		return sentinels.ErrRecordNotFound
	}
	r.images[image.ID] = image
	return nil
}

func (r *fakeImageRepo) FindByID(id uuid.UUID) (*domain.Image, error) {
	image, ok := r.images[id]
	if !ok {
		return nil, sentinels.ErrRecordNotFound
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
	return nil, sentinels.ErrRecordNotFound
}

func (r *fakeImageRepo) FindBySourceURL(sourceURL string) (*domain.Image, error) {
	if r.findBySourceErr != nil {
		return nil, r.findBySourceErr
	}
	if image, ok := r.bySourceURL[sourceURL]; ok {
		return image, nil
	}
	return nil, sentinels.ErrRecordNotFound
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

	// minimal valid JPEG header so DetectContentType returns image/jpeg
	data := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 20)...)
	uploaded, err := svc.PersistUploaded(context.Background(), data, "image/jpeg")

	require.NoError(t, err)
	assert.True(t, len(uploaded.Path) > 0, "path must not be empty")
	assert.Contains(t, string(uploaded.Path), "uploads/")
	// Exactly one file must be saved.
	assert.Len(t, fs.saved, 1)
}

func TestImageService_PersistUploaded_DerivesExtensionFromContentType(t *testing.T) {
	svc, fs, _ := newImageService(t)

	data := make([]byte, 32)
	_, err := svc.PersistUploaded(context.Background(), data, "image/png")

	require.NoError(t, err)
	assert.Len(t, fs.saved, 1)
	for path := range fs.saved {
		assert.True(t, strings.HasSuffix(path, ".png"), "expected .png extension, got %s", path)
	}
}

func TestImageService_SetDefault_MarksImageAsDefault(t *testing.T) {
	svc, _, repo := newImageService(t)

	entityID := uuid.New()
	imgID, _ := uuid.NewV7()
	imgPath := storage.Path("publisher/abc/img.jpg")
	repo.images[imgID] = &domain.Image{
		ID:         imgID,
		EntityType: "publishers",
		EntityID:   entityID,
		Path:       &imgPath,
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
	foodPath := storage.Path("food/abc/img.jpg")
	repo.images[imgID] = &domain.Image{
		ID:         imgID,
		EntityType: "food",
		EntityID:   uuid.New(),
		Path:       &foodPath,
	}

	require.NoError(t, svc.Delete(imgID))

	assert.Contains(t, fs.deleted, "food/abc/img.jpg")
	_, err := repo.FindByID(imgID)
	require.ErrorIs(t, err, sentinels.ErrRecordNotFound)
}
