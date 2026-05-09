package repositories

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type authorRepository struct {
	db *gorm.DB
}

func NewAuthorRepository(db *gorm.DB) domain.AuthorRepository {
	return &authorRepository{db: db}
}

func (r *authorRepository) FindOrCreate(author *domain.Author) error {
	if err := r.find(author); err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(author)
	if result.Error != nil {
		return fmt.Errorf("create author: %w", mapErr(result.Error))
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return r.find(author)
	}

	return nil
}

func (r *authorRepository) find(author *domain.Author) error {
	if author.Url != nil && *author.Url != "" {
		var existing domain.Author
		if err := r.db.First(&existing, "url = ?", author.Url).Error; err == nil {
			*author = existing
			return nil
		}
	}
	if author.Name != "" {
		var existing domain.Author
		if err := r.db.First(&existing, "name = ?", author.Name).Error; err == nil {
			*author = existing
			return nil
		}
	}
	return sentinels.ErrNotFound
}

func (r *authorRepository) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Author, int64, error) {
	q := r.db.Model(&domain.Author{})
	if opts.SearchQuery != "" {
		q = q.Where("name LIKE ?", "%"+opts.SearchQuery+"%")
	}

	q = q.Scopes(RecipeScopeExistsFilter(`
		SELECT 1 FROM recipes
		WHERE recipes.author_id = authors.id
	`, opts.Scope, householdID))

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("search count authors: %w", mapErr(err))
	} else if total == 0 {
		return nil, 0, nil
	}

	sortByRecipes := strings.EqualFold(opts.Sort, "total_recipes")

	selectCols := []string{"authors.*"}
	var selectArgs []any

	if opts.Has("total_recipes") || sortByRecipes {
		if householdID == uuid.Nil {
			selectCols = append(selectCols, `(
				SELECT COUNT(*) FROM recipes
				WHERE recipes.author_id = authors.id
			) AS total_recipes`)
		} else {
			scopeWhere, scopeArgs := scopeWhereArgs(opts.Scope, householdID)
			selectCols = append(selectCols, `(
				SELECT COUNT(*) FROM recipes
				WHERE recipes.author_id = authors.id
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
			Column: clause.Column{Table: "authors", Name: opts.Sort},
			Desc:   strings.EqualFold(opts.Order, "DESC"),
		})
	}

	var authors []domain.Author
	if err := q.Offset(opts.Offset).Limit(opts.Limit).Find(&authors).Error; err != nil {
		return nil, 0, fmt.Errorf("search find authors: %w", mapErr(err))
	}
	return authors, total, nil
}
