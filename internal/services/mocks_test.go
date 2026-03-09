package services_test

import (
	"context"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/storage"
)

type stubRecipeRepo struct {
	domain.RecipeRepository

	byIDFn                    func(uuid.UUID) (*domain.Recipe, error)
	byUrlFn                   func(string) (*domain.Recipe, error)
	byParentIDsAndHouseholdFn func([]uuid.UUID, uuid.UUID) ([]domain.Recipe, error)
	searchFn                  func(uuid.UUID, uuid.UUID, domain.RecipeSearchOptions) ([]domain.Recipe, int64, error)
	createFn                  func(*domain.Recipe) error
	importFn                  func(*domain.Recipe) error
	updateFn                  func(*domain.Recipe) error
	deleteFn                  func(uuid.UUID) error
	userSaveFn                func(uuid.UUID, uuid.UUID, uuid.UUID) error
	userUnsaveFn              func(uuid.UUID, uuid.UUID) error
	createImagesFn            func([]*domain.RecipeImage) error
	updateImageFn             func(*domain.RecipeImage) error
	updateIngredientFn        func(*domain.RecipeIngredient) error
	updateInstructionFn       func(*domain.RecipeInstruction) error
	transactionFn             func(func(domain.RecipeRepository) error) error
	replaceRecipePointersFn   func(uuid.UUID, uuid.UUID, uuid.UUID) error
}

func (s *stubRecipeRepo) ByID(id uuid.UUID) (*domain.Recipe, error) { return s.byIDFn(id) }
func (s *stubRecipeRepo) ByUrl(url string) (*domain.Recipe, error)  { return s.byUrlFn(url) }
func (s *stubRecipeRepo) ByParentIDsAndHousehold(ids []uuid.UUID, hid uuid.UUID) ([]domain.Recipe, error) {
	return s.byParentIDsAndHouseholdFn(ids, hid)
}
func (s *stubRecipeRepo) Search(uid, hid uuid.UUID, opts domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
	return s.searchFn(uid, hid, opts)
}
func (s *stubRecipeRepo) Create(r *domain.Recipe) error          { return s.createFn(r) }
func (s *stubRecipeRepo) Import(r *domain.Recipe) error          { return s.importFn(r) }
func (s *stubRecipeRepo) Update(r *domain.Recipe) error          { return s.updateFn(r) }
func (s *stubRecipeRepo) Delete(id uuid.UUID) error              { return s.deleteFn(id) }
func (s *stubRecipeRepo) UserSave(rid, uid, hid uuid.UUID) error { return s.userSaveFn(rid, uid, hid) }
func (s *stubRecipeRepo) UserUnsave(rid, uid uuid.UUID) error    { return s.userUnsaveFn(rid, uid) }
func (s *stubRecipeRepo) CreateImages(imgs []*domain.RecipeImage) error {
	return s.createImagesFn(imgs)
}
func (s *stubRecipeRepo) UpdateImage(img *domain.RecipeImage) error { return s.updateImageFn(img) }
func (s *stubRecipeRepo) UpdateIngredient(i *domain.RecipeIngredient) error {
	return s.updateIngredientFn(i)
}
func (s *stubRecipeRepo) UpdateInstruction(i *domain.RecipeInstruction) error {
	return s.updateInstructionFn(i)
}
func (s *stubRecipeRepo) Transaction(fn func(domain.RecipeRepository) error) error {
	return s.transactionFn(fn)
}
func (s *stubRecipeRepo) ReplaceRecipePointers(old, newID, hid uuid.UUID) error {
	return s.replaceRecipePointersFn(old, newID, hid)
}

type stubUserRepo struct {
	domain.UserRepository

	byEmailFn     func(string) (*domain.User, error)
	createFn      func(*domain.User) error
	findTokenFn   func(string, string) (*domain.UserToken, error)
	createTokenFn func(*domain.UserToken) error
	deleteTokenFn func(string) error
}

func (s *stubUserRepo) ByEmail(email string) (*domain.User, error) { return s.byEmailFn(email) }
func (s *stubUserRepo) Create(u *domain.User) error                { return s.createFn(u) }
func (s *stubUserRepo) FindToken(tok, typ string) (*domain.UserToken, error) {
	return s.findTokenFn(tok, typ)
}
func (s *stubUserRepo) CreateToken(t *domain.UserToken) error { return s.createTokenFn(t) }
func (s *stubUserRepo) DeleteToken(tok string) error          { return s.deleteTokenFn(tok) }

type stubImageService struct {
	domain.ImageService

	downloadAndSaveFn func(context.Context, string, string) (*domain.UploadedImage, error)
	deleteImageFn     func(storage.Path) error
}

func (s *stubImageService) DownloadAndSaveImage(ctx context.Context, url, path string) (*domain.UploadedImage, error) {
	if s.downloadAndSaveFn != nil {
		return s.downloadAndSaveFn(ctx, url, path)
	}
	return nil, nil
}
func (s *stubImageService) DeleteImage(p storage.Path) error {
	if s.deleteImageFn != nil {
		return s.deleteImageFn(p)
	}
	return nil
}

type stubPublisherService struct {
	domain.PublisherService

	findOrCreateFn func(context.Context, *domain.Publisher) error
}

func (s *stubPublisherService) FindOrCreate(ctx context.Context, pub *domain.Publisher) error {
	if s.findOrCreateFn != nil {
		return s.findOrCreateFn(ctx, pub)
	}
	return nil
}

type stubFoodRepo struct {
	domain.FoodRepository

	findOrCreateFn func(*domain.Food) error
}

func (s *stubFoodRepo) FindOrCreate(f *domain.Food) error {
	if s.findOrCreateFn != nil {
		return s.findOrCreateFn(f)
	}
	return nil
}

type stubUnitRepo struct {
	domain.UnitRepository

	findOrCreateFn func(*domain.Unit) error
}

func (s *stubUnitRepo) FindOrCreate(u *domain.Unit) error {
	if s.findOrCreateFn != nil {
		return s.findOrCreateFn(u)
	}
	return nil
}

type stubScraperService struct {
	domain.ScraperService

	scrapeRecipeFn func(string) (*domain.Recipe, error)
	scrapeFeedFn   func(string, domain.FeedScrapeOptions) ([]*domain.Recipe, error)
}

func (s *stubScraperService) ScrapeRecipe(url string) (*domain.Recipe, error) {
	return s.scrapeRecipeFn(url)
}
func (s *stubScraperService) ScrapeFeed(url string, opts domain.FeedScrapeOptions) ([]*domain.Recipe, error) {
	return s.scrapeFeedFn(url, opts)
}

type stubFeedRepo struct {
	domain.FeedRepository

	listActiveFn func() ([]domain.Feed, error)
	updateFn     func(*domain.Feed) error
}

func (s *stubFeedRepo) ListActive() ([]domain.Feed, error) { return s.listActiveFn() }
func (s *stubFeedRepo) Update(f *domain.Feed) error        { return s.updateFn(f) }

type stubPublisherRepo struct {
	domain.PublisherRepository
}

type stubRecipeService struct {
	domain.RecipeService

	importRecipeFn func(context.Context, *domain.Recipe) (*domain.Recipe, error)
}

func (s *stubRecipeService) ImportRecipe(ctx context.Context, r *domain.Recipe) (*domain.Recipe, error) {
	return s.importRecipeFn(ctx, r)
}

func ptr[T any](v T) *T { return &v }
