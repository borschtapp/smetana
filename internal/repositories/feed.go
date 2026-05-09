package repositories

import (
	"fmt"
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

// junction table for feeds and households
type feedSubscription struct{}

func (feedSubscription) TableName() string {
	return "feed_subscriptions"
}

func NewFeedRepository(db *gorm.DB) domain.FeedRepository {
	return &feedRepository{db: db}
}

func (r *feedRepository) ByIDForHousehold(id uuid.UUID, householdID uuid.UUID) (*domain.Feed, error) {
	var feed domain.Feed
	if err := r.db.
		Select("feeds.*").
		Scopes(FeedSubscribedByHousehold(householdID)).
		First(&feed, id).Error; err != nil {
		return nil, fmt.Errorf("feed by id %s for household %s: %w", id, householdID, mapErr(err))
	}
	return &feed, nil
}

func (r *feedRepository) ByUrl(url string) (*domain.Feed, error) {
	var feed domain.Feed
	if err := r.db.Select("feeds.*").Where("url = ?", url).First(&feed).Error; err != nil {
		return nil, fmt.Errorf("feed by url %s: %w", url, mapErr(err))
	}
	return &feed, nil
}

func (r *feedRepository) ListActive() ([]domain.Feed, error) {
	var feeds []domain.Feed
	if err := r.db.Select("feeds.*").Scopes(ActiveFeed).Find(&feeds).Error; err != nil {
		return nil, fmt.Errorf("list active feeds: %w", mapErr(err))
	}
	return feeds, nil
}

func (r *feedRepository) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Feed, int64, error) {
	var feeds []domain.Feed

	q := r.db.Model(&domain.Feed{}).
		Scopes(FeedSubscribedByHousehold(householdID))

	if opts.SearchQuery != "" {
		q = q.Where("feeds.name LIKE ?", "%"+opts.SearchQuery+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count feeds for household %s: %w", householdID, mapErr(err))
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
		return nil, 0, fmt.Errorf("find feeds for household %s: %w", householdID, mapErr(err))
	}

	if opts.Has("last3_recipes") {
		for i := range feeds {
			if err := r.db.Select("recipes.*").
				Where("feed_id = ?", feeds[i].ID).
				Order("created DESC").
				Limit(3).
				Find(&feeds[i].Recipes).Error; err != nil {
				return nil, 0, fmt.Errorf("find last 3 recipes for feed %s: %w", feeds[i].ID, mapErr(err))
			}
		}
	}

	return feeds, total, nil
}

func (r *feedRepository) AddFeed(householdID uuid.UUID, feed *domain.Feed) error {
	wasActive := feed.Active // snapshot before Append triggers BeforeCreate and mutates the struct
	if err := r.db.Model(&domain.Household{ID: householdID}).Association("Feeds").Append(feed); err != nil {
		return fmt.Errorf("add feed %s to household %s: %w", feed.ID, householdID, mapErr(err))
	}
	if !wasActive {
		if err := r.db.Model(feed).Update("active", true).Error; err != nil {
			return fmt.Errorf("activate feed %s: %w", feed.ID, mapErr(err))
		}
	}
	return nil
}

func (r *feedRepository) DeleteFeed(householdID uuid.UUID, feedID uuid.UUID) error {
	if err := r.db.Model(&domain.Household{ID: householdID}).Association("Feeds").Delete(&domain.Feed{ID: feedID}); err != nil {
		return fmt.Errorf("delete feed association %s for household %s: %w", feedID, householdID, mapErr(err))
	}

	var count int64
	if err := r.db.Model(&feedSubscription{}).Where("feed_id = ?", feedID).Count(&count).Error; err != nil {
		return fmt.Errorf("count subscriptions for feed %s: %w", feedID, mapErr(err))
	}
	if count == 0 {
		if err := r.db.Model(&domain.Feed{}).Where("id = ?", feedID).Update("active", false).Error; err != nil {
			return fmt.Errorf("deactivate feed %s: %w", feedID, mapErr(err))
		}
	}
	return nil
}

func (r *feedRepository) Create(feed *domain.Feed) error {
	if err := r.db.Create(feed).Error; err != nil {
		return fmt.Errorf("create feed: %w", mapErr(err))
	}
	return nil
}

func (r *feedRepository) Update(feed *domain.Feed) error {
	// Explicitly select mutable columns so that zero-value fields like are persisted correctly
	if err := r.db.Model(feed).Select(
		"name",
		"description",
		"url",
		"active",
		"error_count",
		"last_sync_at",
		"last_sync_success",
		"discovered",
	).Updates(feed).Error; err != nil {
		return fmt.Errorf("update feed %s: %w", feed.ID, mapErr(err))
	}
	return nil
}

func (r *feedRepository) Delete(id uuid.UUID) error {
	if err := r.db.Delete(&domain.Feed{}, id).Error; err != nil {
		return fmt.Errorf("delete feed %s: %w", id, mapErr(err))
	}
	return nil
}
