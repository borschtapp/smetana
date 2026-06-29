package repositories

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type taxonomyRepository struct {
	db *gorm.DB
}

func NewTaxonomyRepository(db *gorm.DB) domain.TaxonomyRepository {
	return &taxonomyRepository{db: db}
}

func (r *taxonomyRepository) Search(taxonomyType string, householdID uuid.UUID, opts types.SearchOptions) ([]domain.Taxonomy, int64, error) {
	q := r.db.Model(&domain.Taxonomy{})
	if taxonomyType != "" {
		q = q.Where("type = ?", taxonomyType)
	}

	q = q.Scopes(RecipeScopeExistsFilter(`
		SELECT 1 FROM recipe_taxonomies
		JOIN recipes ON recipes.id = recipe_taxonomies.recipe_id
		WHERE recipe_taxonomies.taxonomy_id = taxonomies.id
	`, opts.Scope, householdID))

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("search count taxonomies type %s: %w", taxonomyType, mapErr(err))
	} else if total == 0 {
		return nil, 0, nil
	}

	sortByRecipes := strings.EqualFold(opts.Sort, "total_recipes")

	selectCols := []string{"taxonomies.*"}
	var selectArgs []any

	if opts.Has("total_recipes") || sortByRecipes {
		if householdID == uuid.Nil {
			selectCols = append(selectCols, `(
				SELECT COUNT(*) FROM recipe_taxonomies
				WHERE recipe_taxonomies.taxonomy_id = taxonomies.id
			) AS total_recipes`)
		} else {
			scopeWhere, scopeArgs := scopeWhereArgs(opts.Scope, householdID)
			selectCols = append(selectCols, `(
				SELECT COUNT(*) FROM recipe_taxonomies
				JOIN recipes ON recipes.id = recipe_taxonomies.recipe_id
				WHERE recipe_taxonomies.taxonomy_id = taxonomies.id
				AND `+scopeWhere+`
			) AS total_recipes`)
			selectArgs = append(selectArgs, scopeArgs...)
		}
	}

	q = q.Select(strings.Join(selectCols, ", "), selectArgs...)

	if sortByRecipes {
		q = q.Order("total_recipes " + opts.Order)
	} else {
		q = q.Order(clause.OrderByColumn{
			Column: clause.Column{Table: "taxonomies", Name: opts.Sort},
			Desc:   strings.EqualFold(opts.Order, "DESC"),
		})
	}

	var taxonomies []domain.Taxonomy
	if err := q.Offset(opts.Offset).Limit(opts.Limit).Find(&taxonomies).Error; err != nil {
		return nil, 0, fmt.Errorf("search find taxonomies type %s: %w", taxonomyType, mapErr(err))
	}
	return taxonomies, total, nil
}

func (r *taxonomyRepository) ByID(id uuid.UUID) (*domain.Taxonomy, error) {
	var t domain.Taxonomy
	if err := r.db.First(&t, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("taxonomy by id %s: %w", id, mapErr(err))
	}
	return &t, nil
}

func (r *taxonomyRepository) ByIDs(ids []uuid.UUID) (map[uuid.UUID]*domain.Taxonomy, error) {
	return byIDs(r.db, ids, func(t *domain.Taxonomy) uuid.UUID { return t.ID })
}

// reassignJunction moves all rows in a taxonomy junction table from mergeID to keepID,
// then deletes whatever is left under mergeID. Rows that would duplicate an existing
// (keepID, taxonomy_id) pair are excluded from the UPDATE instead of relying on an
// upsert, so the statements stay plain SQL that every dialect understands the same way.
func reassignJunction(tx *gorm.DB, table, fkCol string, keepID, mergeID uuid.UUID) error {
	var keptFKs []uuid.UUID
	if err := tx.Table(table).Where("taxonomy_id = ?", keepID).Pluck(fkCol, &keptFKs).Error; err != nil {
		return fmt.Errorf("load kept %s: %w", table, mapErr(err))
	}

	q := tx.Table(table).Where("taxonomy_id = ?", mergeID)
	if len(keptFKs) > 0 {
		q = q.Where(fkCol+" NOT IN ?", keptFKs)
	}
	if err := q.Update("taxonomy_id", keepID).Error; err != nil {
		return fmt.Errorf("reassign %s: %w", table, mapErr(err))
	}

	if err := tx.Table(table).Where("taxonomy_id = ?", mergeID).Delete(nil).Error; err != nil {
		return fmt.Errorf("delete old %s: %w", table, mapErr(err))
	}
	return nil
}

func (r *taxonomyRepository) Merge(keepID, mergeID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var keep, merge domain.Taxonomy
		if err := tx.First(&keep, "id = ? AND canonical_id IS NULL", keepID).Error; err != nil {
			return fmt.Errorf("keep taxonomy: %w", mapErr(err))
		}
		if err := tx.First(&merge, "id = ? AND canonical_id IS NULL", mergeID).Error; err != nil {
			return fmt.Errorf("merge taxonomy: %w", mapErr(err))
		}

		for _, j := range []struct{ table, fkCol string }{
			{"recipe_taxonomies", "recipe_id"},
			{"food_taxonomies", "food_id"},
			{"unit_taxonomies", "unit_id"},
		} {
			if err := reassignJunction(tx, j.table, j.fkCol, keepID, mergeID); err != nil {
				return err
			}
		}

		if err := tx.Model(&domain.Taxonomy{}).Where("id = ?", mergeID).Update("canonical_id", keepID).Error; err != nil {
			return fmt.Errorf("set taxonomy alias: %w", mapErr(err))
		}
		return nil
	})
}

func (r *taxonomyRepository) Detach(id uuid.UUID) error {
	return detach[domain.Taxonomy](r.db, id, "canonical_id")
}

func (r *taxonomyRepository) FindOrCreate(taxonomy *domain.Taxonomy) error {
	if err := r.db.First(taxonomy, "slug = ?", taxonomy.Slug).Error; err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(taxonomy)
	if result.Error != nil {
		return fmt.Errorf("create taxonomy: %w", mapErr(result.Error))
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		if err := r.db.First(taxonomy, "slug = ?", taxonomy.Slug).Error; err != nil {
			return fmt.Errorf("find taxonomy after conflict: %w", mapErr(err))
		}
	}

	return nil
}

func (r *taxonomyRepository) MergeCandidates(since time.Time) (workSet, catalog []domain.Taxonomy, err error) {
	if err := r.db.Where("canonical_id IS NULL AND updated > ?", since).Find(&workSet).Error; err != nil {
		return nil, nil, fmt.Errorf("merge candidates workset: %w", mapErr(err))
	}
	if err := r.db.Where("canonical_id IS NULL").Find(&catalog).Error; err != nil {
		return nil, nil, fmt.Errorf("merge candidates catalog: %w", mapErr(err))
	}
	return workSet, catalog, nil
}
