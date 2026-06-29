package suggestions

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/database"
)

// openPrivateTestDB opens an isolated in-memory SQLite database with the full production
// schema applied, mirroring internal/repositories/recipe_test.go's helper of the same name
// (unexported to that package, so reimplemented here for the suggestions package's tests).
func openPrivateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "failed to open private in-memory SQLite")
	require.NoError(t, database.Migrate(db), "failed to apply migrations")
	return db
}

func seedRecipe(t *testing.T, db *gorm.DB) *domain.Recipe {
	t.Helper()
	r := &domain.Recipe{Name: ptr("Test Recipe")}
	require.NoError(t, db.Create(r).Error)
	return r
}

func seedIngredient(t *testing.T, db *gorm.DB, recipeID uuid.UUID, name string) *domain.RecipeIngredient {
	t.Helper()
	ing := &domain.RecipeIngredient{RecipeID: recipeID, Name: &name, RawText: name}
	require.NoError(t, db.Create(ing).Error)
	return ing
}

func seedFood(t *testing.T, db *gorm.DB, slug, name string) *domain.Food {
	t.Helper()
	f := &domain.Food{Slug: slug, Name: name}
	require.NoError(t, db.Create(f).Error)
	return f
}

func ptr[T any](v T) *T { return &v }
