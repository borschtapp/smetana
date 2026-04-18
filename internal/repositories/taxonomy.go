package repositories

import (
	"strings"

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

	if householdID != uuid.Nil && opts.Scope != "" {
		scopeWhere, scopeArgs := scopeWhereArgs(opts.Scope, householdID)
		q = q.Where(`EXISTS (
			SELECT 1 FROM recipe_taxonomies
			JOIN recipes ON recipes.id = recipe_taxonomies.recipe_id
			WHERE recipe_taxonomies.taxonomy_id = taxonomies.id
			AND `+scopeWhere+`
		)`, scopeArgs...)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, mapErr(err)
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
		return nil, 0, mapErr(err)
	}
	return taxonomies, total, nil
}

func (r *taxonomyRepository) FindOrCreate(taxonomy *domain.Taxonomy) error {
	if err := r.db.First(taxonomy, "slug = ?", taxonomy.Slug).Error; err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(taxonomy)
	if result.Error != nil {
		return mapErr(result.Error)
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return mapErr(r.db.First(taxonomy, "slug = ?", taxonomy.Slug).Error)
	}

	return nil
}
