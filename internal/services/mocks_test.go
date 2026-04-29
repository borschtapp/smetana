package services_test

import (
	"context"

	"borscht.app/smetana/internal/types"
	"github.com/borschtapp/krip"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

type stubRecipeService struct {
	domain.RecipeService

	byIDFn                    func(uuid.UUID, uuid.UUID) (*domain.Recipe, error)
	byUrlFn                   func(string, uuid.UUID) (*domain.Recipe, error)
	byParentIDsAndHouseholdFn func([]uuid.UUID, uuid.UUID, types.PreloadOptions) ([]domain.Recipe, error)
	searchFn                  func(uuid.UUID, uuid.UUID, domain.RecipeSearchOptions) ([]domain.Recipe, int64, error)
	userSaveFn                func(uuid.UUID, uuid.UUID, uuid.UUID) error
	importFn                  func(*domain.Recipe) error
	setFeedIDFn               func(uuid.UUID, uuid.UUID) error
}

func (s *stubRecipeService) ByID(id, householdID uuid.UUID) (*domain.Recipe, error) {
	if s.byIDFn != nil {
		return s.byIDFn(id, householdID)
	}
	return nil, nil
}

func (s *stubRecipeService) ByUrl(url string, householdID uuid.UUID) (*domain.Recipe, error) {
	if s.byUrlFn != nil {
		return s.byUrlFn(url, householdID)
	}
	return nil, nil
}

func (s *stubRecipeService) ByParentIDsAndHousehold(parentIDs []uuid.UUID, householdID uuid.UUID, preload types.PreloadOptions) ([]domain.Recipe, error) {
	if s.byParentIDsAndHouseholdFn != nil {
		return s.byParentIDsAndHouseholdFn(parentIDs, householdID, preload)
	}
	return nil, nil
}

func (s *stubRecipeService) Search(userID, householdID uuid.UUID, opts domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
	if s.searchFn != nil {
		return s.searchFn(userID, householdID, opts)
	}
	return nil, 0, nil
}

func (s *stubRecipeService) UserSave(recipeID, userID, householdID uuid.UUID) error {
	if s.userSaveFn != nil {
		return s.userSaveFn(recipeID, userID, householdID)
	}
	return nil
}

func (s *stubRecipeService) Import(recipe *domain.Recipe) error {
	if s.importFn != nil {
		return s.importFn(recipe)
	}
	return nil
}

func (s *stubRecipeService) SetFeedID(recipeID, feedID uuid.UUID) error {
	if s.setFeedIDFn != nil {
		return s.setFeedIDFn(recipeID, feedID)
	}
	return nil
}

type stubRecipeIngestService struct {
	domain.RecipeIngestService
	importRecipeFn func(context.Context, *domain.Recipe) (*domain.Recipe, error)
}

func (s *stubRecipeIngestService) ImportRecipe(ctx context.Context, recipe *domain.Recipe) (*domain.Recipe, error) {
	if s.importRecipeFn != nil {
		return s.importRecipeFn(ctx, recipe)
	}
	return recipe, nil
}

type stubRecipeRepo struct {
	domain.RecipeRepository

	byIDFn                    func(uuid.UUID) (*domain.Recipe, error)
	byIDPreloadFn             func(uuid.UUID, uuid.UUID, uuid.UUID, types.PreloadOptions) (*domain.Recipe, error)
	byUrlFn                   func(string) (*domain.Recipe, error)
	byParentIDsAndHouseholdFn func([]uuid.UUID, uuid.UUID, types.PreloadOptions) ([]domain.Recipe, error)
	searchFn                  func(uuid.UUID, uuid.UUID, domain.RecipeSearchOptions) ([]domain.Recipe, int64, error)
	createFn                  func(*domain.Recipe) error
	importFn                  func(*domain.Recipe) error
	updateFn                  func(*domain.Recipe) error
	deleteFn                  func(uuid.UUID) error
	userSaveFn                func(uuid.UUID, uuid.UUID, uuid.UUID) error
	userUnsaveFn              func(uuid.UUID, uuid.UUID) error
	updateIngredientFn        func(*domain.RecipeIngredient) error
	deleteIngredientFn        func(uuid.UUID, uuid.UUID) error
	createIngredientFn        func(*domain.RecipeIngredient) error
	addEquipmentFn            func(uuid.UUID, uuid.UUID) error
	removeEquipmentFn         func(uuid.UUID, uuid.UUID) error
	updateInstructionFn       func(*domain.RecipeInstruction) error
	transactionFn             func(func(domain.RecipeRepository) error) error
	replaceRecipePointersFn   func(uuid.UUID, uuid.UUID, uuid.UUID) error
}

func (s *stubRecipeRepo) ByID(id uuid.UUID) (*domain.Recipe, error) { return s.byIDFn(id) }
func (s *stubRecipeRepo) ByIDPreload(id, uid, hid uuid.UUID, p types.PreloadOptions) (*domain.Recipe, error) {
	return s.byIDPreloadFn(id, uid, hid, p)
}
func (s *stubRecipeRepo) ByUrl(url string) (*domain.Recipe, error) { return s.byUrlFn(url) }
func (s *stubRecipeRepo) ByParentIDsAndHousehold(ids []uuid.UUID, hid uuid.UUID, preload types.PreloadOptions) ([]domain.Recipe, error) {
	return s.byParentIDsAndHouseholdFn(ids, hid, preload)
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
func (s *stubRecipeRepo) CreateIngredient(i *domain.RecipeIngredient) error {
	if s.createIngredientFn != nil {
		return s.createIngredientFn(i)
	}
	return nil
}
func (s *stubRecipeRepo) UpdateIngredient(i *domain.RecipeIngredient) error {
	return s.updateIngredientFn(i)
}
func (s *stubRecipeRepo) DeleteIngredient(id, recipeID uuid.UUID) error {
	if s.deleteIngredientFn != nil {
		return s.deleteIngredientFn(id, recipeID)
	}
	return nil
}
func (s *stubRecipeRepo) AddEquipment(recipeID, equipmentID uuid.UUID) error {
	if s.addEquipmentFn != nil {
		return s.addEquipmentFn(recipeID, equipmentID)
	}
	return nil
}
func (s *stubRecipeRepo) RemoveEquipment(recipeID, equipmentID uuid.UUID) error {
	if s.removeEquipmentFn != nil {
		return s.removeEquipmentFn(recipeID, equipmentID)
	}
	return nil
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

	byIDFn        func(uuid.UUID) (*domain.User, error)
	byEmailFn     func(string) (*domain.User, error)
	createFn      func(*domain.User) error
	updateFn      func(*domain.User) error
	deleteFn      func(uuid.UUID) error
	findTokenFn   func(string, string) (*domain.UserToken, error)
	createTokenFn func(*domain.UserToken) error
	deleteTokenFn func(string) (bool, error)
}

func (s *stubUserRepo) ByID(id uuid.UUID) (*domain.User, error) {
	if s.byIDFn != nil {
		return s.byIDFn(id)
	}
	return nil, nil
}
func (s *stubUserRepo) ByEmail(email string) (*domain.User, error) { return s.byEmailFn(email) }
func (s *stubUserRepo) Create(u *domain.User) error                { return s.createFn(u) }
func (s *stubUserRepo) Update(u *domain.User) error {
	if s.updateFn != nil {
		return s.updateFn(u)
	}
	return nil
}
func (s *stubUserRepo) Delete(id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(id)
	}
	return nil
}
func (s *stubUserRepo) FindToken(tok, typ string) (*domain.UserToken, error) {
	return s.findTokenFn(tok, typ)
}
func (s *stubUserRepo) CreateToken(t *domain.UserToken) error { return s.createTokenFn(t) }
func (s *stubUserRepo) DeleteToken(tok string) (bool, error)  { return s.deleteTokenFn(tok) }

type stubHouseholdRepo struct {
	domain.HouseholdRepository

	membersFn func(uuid.UUID, int, int) ([]domain.User, int64, error)
	deleteFn  func(uuid.UUID) error
}

func (s *stubHouseholdRepo) Members(householdID uuid.UUID, offset, limit int) ([]domain.User, int64, error) {
	if s.membersFn != nil {
		return s.membersFn(householdID, offset, limit)
	}
	return nil, 0, nil
}
func (s *stubHouseholdRepo) Delete(id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(id)
	}
	return nil
}

type stubImageService struct {
	domain.ImageService

	persistRemoteFn func(context.Context, *domain.Image, string) error
	deleteFn        func(uuid.UUID) error
	setDefaultFn    func(*domain.Image) error
}

func (s *stubImageService) PersistRemote(ctx context.Context, image *domain.Image, pathPrefix string) error {
	if s.persistRemoteFn != nil {
		return s.persistRemoteFn(ctx, image, pathPrefix)
	}
	return nil
}
func (s *stubImageService) Delete(id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(id)
	}
	return nil
}
func (s *stubImageService) SetDefault(image *domain.Image) error {
	if s.setDefaultFn != nil {
		return s.setDefaultFn(image)
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

type stubAuthorService struct {
	domain.AuthorService

	findOrCreateFn func(context.Context, *domain.Author) error
}

func (s *stubAuthorService) FindOrCreate(ctx context.Context, author *domain.Author) error {
	if s.findOrCreateFn != nil {
		return s.findOrCreateFn(ctx, author)
	}
	return nil
}

type stubFoodService struct {
	domain.FoodService

	findOrCreateFn func(context.Context, *domain.Food) error
	updateFn       func(*domain.Food) error
	latestPricesFn func(uuid.UUID, []uuid.UUID) (map[uuid.UUID]*domain.FoodPrice, error)
}

func (s *stubFoodService) FindOrCreate(ctx context.Context, f *domain.Food) error {
	if s.findOrCreateFn != nil {
		return s.findOrCreateFn(ctx, f)
	}
	return nil
}
func (s *stubFoodService) AddTaxonomy(foodID uuid.UUID, taxonomy *domain.Taxonomy) error {
	return nil
}
func (s *stubFoodService) Update(f *domain.Food) error {
	if s.updateFn != nil {
		return s.updateFn(f)
	}
	return nil
}
func (s *stubFoodService) LatestPrices(householdID uuid.UUID, foodIDs []uuid.UUID) (map[uuid.UUID]*domain.FoodPrice, error) {
	if s.latestPricesFn != nil {
		return s.latestPricesFn(householdID, foodIDs)
	}
	return nil, nil
}

type stubUnitService struct {
	domain.UnitService

	findOrCreateFn func(*domain.Unit) error
	convertFn      func(float64, uuid.UUID, uuid.UUID) (float64, error)
}

func (s *stubUnitService) FindOrCreate(u *domain.Unit) error {
	if s.findOrCreateFn != nil {
		return s.findOrCreateFn(u)
	}
	return nil
}
func (s *stubUnitService) Convert(amount float64, from, to uuid.UUID) (float64, error) {
	if s.convertFn != nil {
		return s.convertFn(amount, from, to)
	}
	return amount, nil
}

type stubTaxonomyRepo struct {
	domain.TaxonomyRepository

	findOrCreateFn func(*domain.Taxonomy) error
}

func (s *stubTaxonomyRepo) FindOrCreate(t *domain.Taxonomy) error {
	if s.findOrCreateFn != nil {
		return s.findOrCreateFn(t)
	}
	return nil
}

type stubEquipmentService struct {
	domain.EquipmentService

	findOrCreateFn func(context.Context, *domain.Equipment) error
}

func (s *stubEquipmentService) FindOrCreate(ctx context.Context, e *domain.Equipment) error {
	if s.findOrCreateFn != nil {
		return s.findOrCreateFn(ctx, e)
	}
	return nil
}

type stubScraperService struct {
	domain.ScraperService

	scrapeUrlFn    func(context.Context, string) (*domain.ScrapeResult, error)
	scrapeRecipeFn func(context.Context, string) (*domain.Recipe, error)
	scrapeFeedFn   func(context.Context, *domain.Feed, krip.FeedOptions) ([]*domain.Recipe, error)
}

func (s *stubScraperService) ScrapeUrl(ctx context.Context, url string) (*domain.ScrapeResult, error) {
	if s.scrapeUrlFn != nil {
		return s.scrapeUrlFn(ctx, url)
	}
	return nil, nil
}
func (s *stubScraperService) ScrapeRecipe(ctx context.Context, url string) (*domain.Recipe, error) {
	if s.scrapeRecipeFn != nil {
		return s.scrapeRecipeFn(ctx, url)
	}
	return nil, nil
}
func (m *stubScraperService) ScrapeFeed(ctx context.Context, feed *domain.Feed, opts krip.FeedOptions) ([]*domain.Recipe, error) {
	if m.scrapeFeedFn != nil {
		return m.scrapeFeedFn(ctx, feed, opts)
	}
	return nil, nil
}

type stubFeedRepo struct {
	domain.FeedRepository

	listActiveFn func() ([]domain.Feed, error)
	updateFn     func(*domain.Feed) error
}

func (s *stubFeedRepo) ListActive() ([]domain.Feed, error) { return s.listActiveFn() }
func (s *stubFeedRepo) Update(f *domain.Feed) error        { return s.updateFn(f) }

type stubFeedService struct {
	domain.FeedService

	subscribeFn func(context.Context, uuid.UUID, string, *domain.Feed) (*domain.Feed, error)
}

func (s *stubFeedService) Subscribe(ctx context.Context, householdID uuid.UUID, url string, scraped *domain.Feed) (*domain.Feed, error) {
	if s.subscribeFn != nil {
		return s.subscribeFn(ctx, householdID, url, scraped)
	}
	return nil, nil
}

type stubTaxonomyService struct {
	domain.TaxonomyService
	findOrCreateFn func(*domain.Taxonomy) error
}

func (s *stubTaxonomyService) FindOrCreate(t *domain.Taxonomy) error {
	if s.findOrCreateFn != nil {
		return s.findOrCreateFn(t)
	}
	return nil
}

func ptr[T any](v T) *T { return &v }
