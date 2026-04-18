package repositories

import (
	"strings"

	"borscht.app/smetana/internal/sentinels"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type publisherRepository struct {
	db *gorm.DB
}

func NewPublisherRepository(db *gorm.DB) domain.PublisherRepository {
	return &publisherRepository{db: db}
}

func (r *publisherRepository) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Publisher, int64, error) {
	q := r.db.Model(&domain.Publisher{})

	if opts.SearchQuery != "" {
		q = q.Where("name LIKE ?", "%"+opts.SearchQuery+"%")
	}

	if householdID != uuid.Nil && opts.Scope != "" {
		scopeWhere, scopeArgs := scopeWhereArgs(opts.Scope, householdID)
		q = q.Where(`EXISTS (
			SELECT 1 FROM recipes
			WHERE recipes.publisher_id = publishers.id
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

	selectCols := []string{"publishers.*"}
	var selectArgs []any

	if opts.Has("total_recipes") || sortByRecipes {
		if householdID == uuid.Nil { // overall total, all households
			selectCols = append(selectCols, `(
				SELECT COUNT(*) FROM recipes
				WHERE recipes.publisher_id = publishers.id
			) AS total_recipes`)
		} else {
			scopeWhere, scopeArgs := scopeWhereArgs(opts.Scope, householdID)
			selectCols = append(selectCols, `(
				SELECT COUNT(*) FROM recipes
				WHERE recipes.publisher_id = publishers.id
				AND `+scopeWhere+`
			) AS total_recipes`)
			selectArgs = append(selectArgs, scopeArgs...)
		}
	}

	q = q.Select(strings.Join(selectCols, ", "), selectArgs...)

	if opts.Has("feeds") {
		q = q.Preload("Feeds")
	}
	if opts.Has("images") {
		q = q.Preload("Images")
	}

	if sortByRecipes {
		q = q.Order("total_recipes " + opts.Order)
	} else {
		q = q.Order(clause.OrderByColumn{
			Column: clause.Column{Table: "publishers", Name: opts.Sort},
			Desc:   strings.EqualFold(opts.Order, "DESC"),
		})
	}

	q = q.Offset(opts.Offset).Limit(opts.Limit)

	var publishers []domain.Publisher
	if err := q.Find(&publishers).Error; err != nil {
		return nil, 0, mapErr(err)
	}

	if opts.Has("last3_recipes") {
		for i := range publishers {
			if err := r.db.Select("recipes.*").
				Where("publisher_id = ?", publishers[i].ID).
				Order("created DESC").
				Limit(3).
				Find(&publishers[i].Recipes).Error; err != nil {
				return nil, 0, mapErr(err)
			}
		}
	}

	return publishers, total, nil
}

func (r *publisherRepository) FindOrCreate(pub *domain.Publisher) error {
	if err := r.find(pub); err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(pub)
	if result.Error != nil {
		return mapErr(result.Error)
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return r.find(pub)
	}

	return nil
}

func (r *publisherRepository) find(pub *domain.Publisher) error {
	if pub.Url != nil && len(*pub.Url) > 0 {
		var existing domain.Publisher
		if err := r.db.First(&existing, "url = ?", pub.Url).Error; err == nil {
			*pub = existing
			return nil
		}
	}
	if len(pub.Name) > 0 {
		var existing domain.Publisher
		if err := r.db.First(&existing, "lower(name) = lower(?)", pub.Name).Error; err == nil {
			*pub = existing
			return nil
		}
	}
	return sentinels.ErrNotFound
}
