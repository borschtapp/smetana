package repositories

import (
	"slices"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type feedRepository struct {
	db *gorm.DB
}

func NewFeedRepository(db *gorm.DB) domain.FeedRepository {
	return &feedRepository{db: db}
}

func (r *feedRepository) ByUrl(url string) (*domain.Feed, error) {
	var feed domain.Feed
	if err := r.db.Where("url = ?", url).First(&feed).Error; err != nil {
		return nil, mapErr(err)
	}
	return &feed, nil
}

func (r *feedRepository) ListActive() ([]domain.Feed, error) {
	var feeds []domain.Feed
	if err := r.db.Select("feeds.*").Where("active = ?", true).Find(&feeds).Error; err != nil {
		return nil, err
	}
	return feeds, nil
}

func (r *feedRepository) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Feed, int64, error) {
	var feeds []domain.Feed

	q := r.db.Model(&domain.Feed{}).
		Joins("JOIN feed_subscriptions ON feed_subscriptions.feed_id = feeds.id").
		Where("feed_subscriptions.household_id = ?", householdID)

	if opts.SearchQuery != "" {
		q = q.Where("feeds.name LIKE ?", "%"+opts.SearchQuery+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	} else if total == 0 {
		return nil, 0, nil
	}

	q = q.Select("feeds.*")

	if len(opts.Preload) != 0 {
		if slices.Contains(opts.Preload, "publisher") {
			q = q.Preload("Publisher")
		}

		if slices.Contains(opts.Preload, "recipes:5") {
			q = q.Preload("Recipes", func(db *gorm.DB) *gorm.DB {
				return db.Order("created DESC").Limit(5)
			})
		}

		if slices.Contains(opts.Preload, "recipes.images") {
			q = q.Preload("Recipes.Images")
		}

		if slices.Contains(opts.Preload, "total_recipes") {
			q = q.Select(`feeds.*, (
					SELECT COUNT(*) FROM recipes
					WHERE recipes.feed_id = feeds.id
				) AS total_recipes`)
		}
	}

	q = q.Offset(opts.Offset).Limit(opts.Limit)
	q = q.Order(clause.OrderByColumn{
		Column: clause.Column{Table: "feeds", Name: opts.Sort},
		Desc:   strings.EqualFold(opts.Order, "DESC"),
	})

	if err := q.Find(&feeds).Error; err != nil {
		return nil, 0, err
	}
	return feeds, total, nil
}

func (r *feedRepository) AddFeed(householdID uuid.UUID, feed *domain.Feed) error {
	return r.db.Model(&domain.Household{ID: householdID}).Association("Feeds").Append(feed)
}

func (r *feedRepository) DeleteFeed(householdID uuid.UUID, feedID uuid.UUID) error {
	return r.db.Model(&domain.Household{ID: householdID}).Association("Feeds").Delete(&domain.Feed{ID: feedID})
}

func (r *feedRepository) Create(feed *domain.Feed) error {
	return r.db.Create(feed).Error
}

func (r *feedRepository) Update(feed *domain.Feed) error {
	// Explicitly select mutable columns so that zero-value fields like are persisted correctly
	return r.db.Model(feed).Select("active", "error_count", "last_sync_at", "last_sync_success", "name").Updates(feed).Error
}

func (r *feedRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Feed{}, id).Error
}
