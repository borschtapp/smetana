package repositories

import (
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
		return nil, mapErr(err)
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
		return nil, 0, mapErr(err)
	} else if total == 0 {
		return nil, 0, nil
	}

	q = q.Select("feeds.*")

	if len(opts.Preload) != 0 {
		if opts.Has("publisher") {
			q = q.Preload("Publisher")
		}

		if opts.Has("total_recipes") {
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
		return nil, 0, mapErr(err)
	}

	if opts.Has("last3_recipes") {
		for i := range feeds {
			if err := r.db.Select("recipes.*").
				Where("feed_id = ?", feeds[i].ID).
				Order("created DESC").
				Limit(3).
				Find(&feeds[i].Recipes).Error; err != nil {
				return nil, 0, mapErr(err)
			}
		}
	}

	return feeds, total, nil
}

func (r *feedRepository) AddFeed(householdID uuid.UUID, feed *domain.Feed) error {
	return mapErr(r.db.Model(&domain.Household{ID: householdID}).Association("Feeds").Append(feed))
}

func (r *feedRepository) DeleteFeed(householdID uuid.UUID, feedID uuid.UUID) error {
	return mapErr(r.db.Model(&domain.Household{ID: householdID}).Association("Feeds").Delete(&domain.Feed{ID: feedID}))
}

func (r *feedRepository) Create(feed *domain.Feed) error {
	return mapErr(r.db.Create(feed).Error)
}

func (r *feedRepository) Update(feed *domain.Feed) error {
	// Explicitly select mutable columns so that zero-value fields like are persisted correctly
	return mapErr(r.db.Model(feed).Select("active", "error_count", "last_sync_at", "last_sync_success", "name").Updates(feed).Error)
}

func (r *feedRepository) Delete(id uuid.UUID) error {
	return mapErr(r.db.Delete(&domain.Feed{}, id).Error)
}
