package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/model"
	"github.com/borschtapp/krip/scraper"
	kUtils "github.com/borschtapp/krip/utils"
	"github.com/doyensec/safeurl"
	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type scraperService struct{}

func NewScraperService() domain.ScraperService {
	return &scraperService{}
}

func defaultRequestOptions(ctx context.Context) krip.RequestOptions {
	return krip.RequestOptions{
		Context:    ctx,
		HttpClient: safeurl.Client(safeurl.GetConfigBuilder().Build()),
	}
}

func (s *scraperService) ScrapeUrl(ctx context.Context, url string) (*domain.ScrapeResult, error) {
	scrapeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	options := krip.FeedOptions{
		ScrapeOptions: krip.ScrapeOptions{
			RequestOptions: defaultRequestOptions(scrapeCtx),
		},
	}

	data, err := scraper.UrlInput(url, options.ScrapeOptions)
	if err != nil {
		return nil, err
	}

	kripRecipe := &krip.Recipe{}
	if err := scraper.Scrape(data, kripRecipe, options.ScrapeOptions); err != nil {
		log.Infow("failed to scrape", "url", url, "error", err.Error())
	}

	if kripRecipe.IsValid() {
		return &domain.ScrapeResult{Type: domain.PageTypeRecipe, Recipe: s.kripToRecipe(kripRecipe)}, nil
	}

	kripFeed := &model.Feed{}
	if err := scraper.ScrapeFeed(data, kripFeed, options); err == nil && len(kripFeed.Entries) > 0 {
		return &domain.ScrapeResult{Type: domain.PageTypeFeed, Feed: s.kripToFeed(kripFeed)}, nil
	}

	// fallback to weak recipe
	if kripRecipe.Name != "" {
		return &domain.ScrapeResult{Type: domain.PageTypeRecipe, Recipe: s.kripToRecipe(kripRecipe)}, nil
	}

	return nil, errors.New("auto: page type could not be determined")
}

func (s *scraperService) ScrapeRecipe(ctx context.Context, url string) (*domain.Recipe, error) {
	scrapeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	kripRecipe, err := krip.ScrapeUrl(url, krip.ScrapeOptions{
		RequestOptions: defaultRequestOptions(scrapeCtx),
	})
	if err != nil {
		return nil, err
	}
	return s.kripToRecipe(kripRecipe), nil
}

func (s *scraperService) ScrapeFeed(ctx context.Context, feed *domain.Feed, opts krip.FeedOptions) ([]*domain.Recipe, error) {
	scrapeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	opts.RequestOptions = defaultRequestOptions(scrapeCtx)
	scrapedFeed, err := krip.ScrapeFeedUrl(feed.Url, opts)
	if err != nil {
		return nil, err
	}

	recipes := make([]*domain.Recipe, 0, len(scrapedFeed.Entries))
	for _, entry := range scrapedFeed.Entries {
		recipes = append(recipes, s.kripToRecipe(entry))
	}

	// Back-populate the feed with scraped metadata.
	if scrapedFeed.Name != "" && feed.Name != scrapedFeed.Name {
		feed.Name = scrapedFeed.Name
	}
	if scrapedFeed.Url != "" && feed.Url != scrapedFeed.Url {
		feed.Url = utils.NormalizeURL(scrapedFeed.Url)
	}
	if scrapedFeed.Publisher != nil && feed.Publisher == nil {
		feed.Publisher = s.kripToPublisher(scrapedFeed.Publisher)
	}
	if scrapedFeed.Description != "" && (feed.Description == nil || *feed.Description != scrapedFeed.Description) {
		feed.Description = new(scrapedFeed.Description)
	}
	if scrapedFeed.Discovered != nil {
		feed.Discovered = scrapedFeed.Discovered
	}

	return recipes, nil
}

func (s *scraperService) enrichIngredients(ingredients []*domain.RecipeIngredient, language string) {
	for _, ingredient := range ingredients {
		s.enrichIngredient(ingredient, language)
	}
}

// enrichIngredient uses Kapusta to parse RawText for ingredients that were structured by Krip.
func (s *scraperService) enrichIngredient(ingredient *domain.RecipeIngredient, language string) {
	if ingredient.Food != nil {
		return // structured fields already set during conversion
	}

	parsed, err := kapusta.ParseIngredient(ingredient.RawText, kapusta.IngredientOptions{Lang: language})
	if err != nil {
		return
	}

	ingredient.Amount = &parsed.Amount
	if parsed.MaxAmount != 0 {
		ingredient.MaxAmount = &parsed.MaxAmount
	}
	if parsed.Unit != "" {
		ingredient.Unit = &domain.Unit{Name: parsed.Unit, Slug: utils.CreateTag(parsed.UnitCode)}
	}
	if parsed.Name != "" {
		ingredient.Name = &parsed.Name
		ingredient.Food = &domain.Food{Name: parsed.Name, Slug: utils.CreateTag(parsed.Name)}
	}
	if parsed.Description != "" {
		ingredient.Description = &parsed.Description
	}
}

func (s *scraperService) kripToRecipe(kripRecipe *krip.Recipe) *domain.Recipe {
	recipe := &domain.Recipe{}
	recipe.SourceUrl = new(utils.NormalizeURL(kripRecipe.Url))
	if kripRecipe.Name != "" {
		recipe.Name = &kripRecipe.Name
	}
	if kripRecipe.Description != "" {
		recipe.Description = &kripRecipe.Description
	}
	if kripRecipe.Language != "" {
		recipe.Language = &kripRecipe.Language
	}
	for _, image := range kripRecipe.Images {
		if image.Url != "" {
			recipe.Images = append(recipe.Images, s.kripToImage(image))
		}
	}
	if kripRecipe.Author != nil && kripRecipe.Author.Name != "" {
		recipe.Author = s.kripToAuthor(kripRecipe.Author)
	}
	if kripRecipe.Text != "" {
		recipe.Text = &kripRecipe.Text
	}
	if kripRecipe.PrepTime != "" {
		if d, err := types.DurationFromISO8601(kripRecipe.PrepTime); err == nil {
			recipe.PrepTime = &d
		}
	}
	if kripRecipe.CookTime != "" {
		if d, err := types.DurationFromISO8601(kripRecipe.CookTime); err == nil {
			recipe.CookTime = &d
		}
	}
	if kripRecipe.TotalTime != "" {
		if d, err := types.DurationFromISO8601(kripRecipe.TotalTime); err == nil {
			recipe.TotalTime = &d
		}
	}
	if kripRecipe.Difficulty != "" {
		recipe.Difficulty = &kripRecipe.Difficulty
	}
	if kripRecipe.CookingMethod != "" {
		recipe.Method = &kripRecipe.CookingMethod
	}
	addTaxonomies(recipe, domain.TaxonomyTypeDiet, kripRecipe.Diets)
	addTaxonomies(recipe, domain.TaxonomyTypeCategory, kripRecipe.Categories)
	addTaxonomies(recipe, domain.TaxonomyTypeCuisine, kripRecipe.Cuisines)
	addTaxonomies(recipe, domain.TaxonomyTypeKeyword, kripRecipe.Keywords)
	if kripRecipe.Yield != "" {
		if yield := kUtils.FindInt(kripRecipe.Yield); yield > 0 {
			recipe.Yield = &yield
		}
	}
	if kripRecipe.Nutrition != nil {
		recipe.Nutrition = &domain.RecipeNutrition{
			ServingSize: kripRecipe.Nutrition.ServingSize,
			Calories:    kripRecipe.Nutrition.Calories,
			Carbs:       kripRecipe.Nutrition.CarbohydrateContent,
			CarbFiber:   kripRecipe.Nutrition.FiberContent,
			CarbSugar:   kripRecipe.Nutrition.SugarContent,
			Cholesterol: kripRecipe.Nutrition.CholesterolContent,
			Sodium:      kripRecipe.Nutrition.SodiumContent,
			Fats:        kripRecipe.Nutrition.FatContent,
			FatSat:      kripRecipe.Nutrition.SaturatedFatContent,
			FatTrans:    kripRecipe.Nutrition.TransFatContent,
			Protein:     kripRecipe.Nutrition.ProteinContent,
			Salt:        kripRecipe.Nutrition.SaltContent,
			Iron:        kripRecipe.Nutrition.IronContent,
			Potassium:   kripRecipe.Nutrition.PotassiumContent,
			Calcium:     kripRecipe.Nutrition.CalciumContent,
		}
	}
	if kripRecipe.Rating != nil {
		recipe.Rating = &domain.Rating{}
		if kripRecipe.Rating.ReviewCount > 0 {
			recipe.Rating.Reviews = &kripRecipe.Rating.ReviewCount
		}
		if kripRecipe.Rating.RatingCount > 0 {
			recipe.Rating.Count = &kripRecipe.Rating.RatingCount
		}
		if kripRecipe.Rating.RatingValue > 0 {
			recipe.Rating.Value = &kripRecipe.Rating.RatingValue
		}
	}
	if kripRecipe.Video != nil {
		recipe.Video = &domain.Video{}
		if kripRecipe.Video.Name != "" {
			recipe.Video.Name = &kripRecipe.Video.Name
		}
		if kripRecipe.Video.Description != "" {
			recipe.Video.Description = &kripRecipe.Video.Description
		}
		if kripRecipe.Video.EmbedUrl != "" {
			recipe.Video.EmbedUrl = &kripRecipe.Video.EmbedUrl
		}
		if kripRecipe.Video.ContentUrl != "" {
			recipe.Video.ContentUrl = &kripRecipe.Video.ContentUrl
		}
		if kripRecipe.Video.ThumbnailUrl != "" {
			recipe.Video.ThumbnailUrl = &kripRecipe.Video.ThumbnailUrl
		}
	}
	if kripRecipe.Publisher != nil {
		recipe.Publisher = s.kripToPublisher(kripRecipe.Publisher)
	}
	if kripRecipe.DateModified != nil {
		recipe.Published = kripRecipe.DateModified
	} else if kripRecipe.DatePublished != nil {
		recipe.Published = kripRecipe.DatePublished
	}
	for _, item := range kripRecipe.Ingredients {
		recipe.Ingredients = append(recipe.Ingredients, s.kripToIngredient(item))
	}
	for _, item := range kripRecipe.Equipment {
		eq := &domain.Equipment{
			Name: item.Name,
			Slug: utils.CreateTag(item.Name),
		}
		if item.Description != "" {
			eq.Description = &item.Description
		}
		if item.Image != "" {
			eq.Images = []*domain.Image{{SourceURL: item.Image}}
		}
		recipe.Equipment = append(recipe.Equipment, eq)
	}
	for _, item := range kripRecipe.Instructions {
		recipe.Instructions = append(recipe.Instructions, s.kripToInstruction(&item.HowToStep))
		for _, step := range item.Steps {
			recipe.Instructions = append(recipe.Instructions, s.kripToInstruction(step))
		}
	}
	s.enrichIngredients(recipe.Ingredients, kripRecipe.Language)
	return recipe
}

func addTaxonomies(recipe *domain.Recipe, taxType string, labels []string) {
	for _, label := range labels {
		recipe.Taxonomies = append(recipe.Taxonomies, &domain.Taxonomy{
			Type:  taxType,
			Label: label,
			Slug:  utils.CreateTag(label),
		})
	}
}

func (s *scraperService) kripToAuthor(person *krip.Person) *domain.Author {
	author := &domain.Author{
		Name: person.Name,
	}
	if person.Url != "" {
		author.Url = new(utils.NormalizeURL(person.Url))
	}
	if person.Description != "" {
		author.Description = &person.Description
	}
	if person.Image != "" {
		author.Images = []*domain.Image{{SourceURL: person.Image}}
	}
	return author
}

func (s *scraperService) kripToImage(image *krip.ImageObject) *domain.Image {
	img := &domain.Image{
		SourceURL: image.Url,
	}
	if image.Width > 0 {
		img.Width = &image.Width
	}
	if image.Height > 0 {
		img.Height = &image.Height
	}
	if image.Caption != "" {
		img.Caption = &image.Caption
	}
	return img
}

func (s *scraperService) kripToInstruction(item *krip.HowToStep) *domain.RecipeInstruction {
	ins := &domain.RecipeInstruction{}
	if item.Name != "" {
		ins.Title = &item.Name
	}
	if item.Text != "" {
		ins.Text = item.Text
	}
	if item.Url != "" {
		ins.Url = new(utils.NormalizeURL(item.Url))
	}
	if item.Image != "" {
		ins.Images = []*domain.Image{{SourceURL: item.Image}}
	}
	if item.Video != "" {
		ins.VideoUrl = &item.Video
	}
	return ins
}

func (s *scraperService) kripToIngredient(item *krip.PropertyValue) *domain.RecipeIngredient {
	ing := &domain.RecipeIngredient{}
	if item.Value != "" {
		// Structured: value, optional unit, and name are separate.
		parts := []string{item.Value}
		if item.UnitText != "" {
			parts = append(parts, item.UnitText)
		}
		parts = append(parts, item.Name)
		ing.RawText = strings.Join(parts, " ")

		if q, err := kUtils.ParseFloat(item.Value); err == nil {
			ing.Amount = &q
		}
		if item.UnitText != "" {
			ing.Unit = &domain.Unit{Name: item.UnitText}
		}
		if item.Name != "" {
			ing.Name = &item.Name
			ing.Food = &domain.Food{Name: item.Name, Slug: utils.CreateTag(item.Name)}
		}
	} else if item.UnitText != "" {
		// Partially: unit and name are known, amount is not.
		ing.RawText = item.UnitText + " " + item.Name
		ing.Unit = &domain.Unit{Name: item.UnitText}
		if item.Name != "" {
			ing.Name = &item.Name
			ing.Food = &domain.Food{Name: item.Name, Slug: utils.CreateTag(item.Name)}
		}
	} else {
		// Unstructured: Name holds the full ingredient string (e.g. "2 cups flour").
		ing.RawText = item.Name
	}

	if item.Pantry && ing.Food != nil {
		ing.Food.Pantry = item.Pantry
	}
	if item.Image != "" && ing.Food != nil {
		ing.Food.Images = []*domain.Image{{SourceURL: item.Image}}
	}

	return ing
}

func (s *scraperService) kripToFeed(f *model.Feed) *domain.Feed {
	feed := &domain.Feed{
		Url:        utils.NormalizeURL(f.Url),
		Name:       f.Name,
		Discovered: f.Discovered,
	}
	if f.Description != "" {
		feed.Description = &f.Description
	}
	if f.Publisher != nil {
		feed.Publisher = s.kripToPublisher(f.Publisher)
	}
	return feed
}

func (s *scraperService) kripToPublisher(org *krip.Organization) *domain.Publisher {
	pub := &domain.Publisher{Name: org.Name}
	if org.Description != "" {
		pub.Description = &org.Description
	}
	if org.Url != "" {
		pub.Url = new(utils.NormalizeURL(org.Url))
	}
	if org.Logo != "" {
		pub.Images = []*domain.Image{{SourceURL: org.Logo}}
	}
	return pub
}
