package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

type FeedRepository struct {
	db *gorm.DB
}

func NewFeedRepository(db *gorm.DB) domain.FeedRepository {
	return &FeedRepository{db: db}
}

func (r *FeedRepository) ByUrl(url string) (*domain.Feed, error) {
	var feed domain.Feed
	if err := r.db.Where("url = ?", url).First(&feed).Error; err != nil {
		return nil, mapErr(err)
	}
	return &feed, nil
}

func (r *FeedRepository) ListActive() ([]domain.Feed, error) {
	var feeds []domain.Feed
	if err := r.db.Where("active = ?", true).Find(&feeds).Error; err != nil {
		return nil, err
	}
	return feeds, nil
}

func (r *FeedRepository) List(householdID uuid.UUID, offset, limit int) ([]domain.Feed, int64, error) {
	var feeds []domain.Feed
	baseQuery := r.db.
		Joins("JOIN feed_subscriptions ON feed_subscriptions.feed_id = feeds.id").
		Where("feed_subscriptions.household_id = ?", householdID)

	var total int64
	if err := baseQuery.Model(&domain.Feed{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := baseQuery.
		Offset(offset).
		Limit(limit).
		Find(&feeds).Error
	return feeds, total, err
}

func (r *FeedRepository) Stream(householdID uuid.UUID, offset, limit int) ([]domain.Recipe, int64, error) {
	var recipes []domain.Recipe

	baseQuery := r.db.
		Joins("JOIN feeds ON feeds.id = recipes.feed_id").
		Joins("JOIN feed_subscriptions ON feed_subscriptions.feed_id = feeds.id").
		Where("feed_subscriptions.household_id = ?", householdID)

	var total int64
	if err := baseQuery.Model(&domain.Recipe{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := baseQuery.
		Preload("Images").
		Order("recipes.created DESC").
		Limit(limit).
		Offset(offset).
		Find(&recipes).Error

	return recipes, total, err
}

func (r *FeedRepository) AddFeed(householdID uuid.UUID, feed *domain.Feed) error {
	return r.db.Model(&domain.Household{ID: householdID}).Association("Feeds").Append(feed)
}

func (r *FeedRepository) DeleteFeed(householdID uuid.UUID, feedID uuid.UUID) error {
	return r.db.Model(&domain.Household{ID: householdID}).Association("Feeds").Delete(&domain.Feed{ID: feedID})
}

func (r *FeedRepository) Create(feed *domain.Feed) error {
	return r.db.Create(feed).Error
}

func (r *FeedRepository) Update(feed *domain.Feed) error {
	// Explicitly select mutable columns so that zero-value fields like are persisted correctly
	return r.db.Model(feed).Select("active", "error_count", "retrieved", "name").Updates(feed).Error
}

func (r *FeedRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Feed{}, id).Error
}
