package repositories_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/storage"
)

func makeImage(entityType string, entityID uuid.UUID) *domain.Image {
	id, _ := uuid.NewV7()
	return &domain.Image{
		ID:          id,
		EntityType:  entityType,
		EntityID:    entityID,
		Path:        new(storage.Path(entityType + "/" + entityID.String() + "/" + id.String() + ".jpg")),
		ContentType: "image/jpeg",
	}
}

func TestImageRepository_CreateAndFindByID(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewImageRepository(db)

	entityID := uuid.New()
	image := makeImage("food", entityID)

	require.NoError(t, repo.Create(image))

	got, err := repo.FindByID(image.ID)
	require.NoError(t, err)
	assert.Equal(t, image.ID, got.ID)
	assert.Equal(t, "food", got.EntityType)
	assert.Equal(t, entityID, got.EntityID)
}

func TestImageRepository_FindByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewImageRepository(db)

	_, err := repo.FindByID(uuid.New())
	require.ErrorIs(t, err, sentinels.ErrRecordNotFound)
}

func TestImageRepository_FindByEntity_ReturnsAllForEntity(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewImageRepository(db)

	entityID := uuid.New()
	img1 := makeImage("food", entityID)
	img2 := makeImage("food", entityID)
	other := makeImage("food", uuid.New()) // different entity

	require.NoError(t, repo.Create(img1))
	require.NoError(t, repo.Create(img2))
	require.NoError(t, repo.Create(other))

	results, err := repo.FindByEntity("food", entityID)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestImageRepository_FindBySourceURL_Deduplication(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewImageRepository(db)

	entityID := uuid.New()
	image := makeImage("publishers", entityID)
	image.SourceURL = "https://example.com/logo.png"
	require.NoError(t, repo.Create(image))

	got, err := repo.FindBySourceURL("https://example.com/logo.png")
	require.NoError(t, err)
	assert.Equal(t, image.ID, got.ID)
}

func TestImageRepository_FindBySourceURL_EmptyString_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewImageRepository(db)

	_, err := repo.FindBySourceURL("")
	require.ErrorIs(t, err, sentinels.ErrRecordNotFound)
}

func TestImageRepository_SetDefault_ClearsPreviousAndSetsNew(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewImageRepository(db)

	entityID := uuid.New()
	first := makeImage("recipes", entityID)
	second := makeImage("recipes", entityID)
	require.NoError(t, repo.Create(first))
	require.NoError(t, repo.Create(second))

	// Set first as default.
	require.NoError(t, repo.SetDefault(first))
	got, err := repo.FindDefault("recipes", entityID)
	require.NoError(t, err)
	assert.Equal(t, first.ID, got.ID)

	// Switch default to second — first must be cleared.
	require.NoError(t, repo.SetDefault(second))
	got, err = repo.FindDefault("recipes", entityID)
	require.NoError(t, err)
	assert.Equal(t, second.ID, got.ID)

	// Verify first is no longer default.
	first2, err := repo.FindByID(first.ID)
	require.NoError(t, err)
	assert.False(t, first2.IsDefault)
}

func TestImageRepository_Delete_RemovesRecord(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewImageRepository(db)

	entityID := uuid.New()
	image := makeImage("food", entityID)
	require.NoError(t, repo.Create(image))
	require.NoError(t, repo.Delete(image.ID))

	_, err := repo.FindByID(image.ID)
	require.ErrorIs(t, err, sentinels.ErrRecordNotFound)
}
