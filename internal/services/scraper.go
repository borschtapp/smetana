package services

import (
	"strings"

	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/model"
	kUtils "github.com/borschtapp/krip/utils"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type ScraperService struct{}

func NewScraperService() domain.ScraperService {
	return &ScraperService{}
}

func (s *ScraperService) ScrapeRecipe(url string) (*domain.Recipe, error) {
	kripRecipe, err := krip.ScrapeUrl(url)
	if err != nil {
		return nil, err
	}
	recipe := s.kripToRecipe(kripRecipe)
	s.enrichIngredients(recipe.Ingredients, kripRecipe.Language)
	return recipe, nil
}

func (s *ScraperService) ScrapeFeed(url string, opts domain.FeedScrapeOptions) ([]*domain.Recipe, error) {
	scrapedFeed, err := krip.ScrapeFeedUrl(url, model.FeedOptions{
		Quick:               opts.Quick,
		MinIngredients:      opts.MinIngredients,
		RequireImage:        opts.RequireImage,
		RequireInstructions: opts.RequireInstructions,
	})
	if err != nil {
		return nil, err
	}

	recipes := make([]*domain.Recipe, 0, len(scrapedFeed.Entries))
	for _, entry := range scrapedFeed.Entries {
		recipe := s.kripToRecipe(entry)
		s.enrichIngredients(recipe.Ingredients, entry.Language)
		recipes = append(recipes, recipe)
	}
	return recipes, nil
}

func (s *ScraperService) enrichIngredients(ingredients []*domain.RecipeIngredient, language string) {
	for _, ingredient := range ingredients {
		s.enrichIngredient(ingredient, language)
	}
}

// enrichIngredient uses Kapusta to parse RawText for ingredients that were structured by Krip.
func (s *ScraperService) enrichIngredient(ingredient *domain.RecipeIngredient, language string) {
	if ingredient.Food != nil {
		return // structured fields already set during conversion
	}
	parsed, err := kapusta.ParseIngredient(ingredient.RawText, language)
	if err != nil || parsed == nil {
		return
	}
	ingredient.Amount = &parsed.Quantity
	if len(parsed.Annotation) != 0 {
		ingredient.Note = &parsed.Annotation
	}
	if len(parsed.Ingredient) != 0 {
		ingredient.Food = &domain.Food{Name: parsed.Ingredient}
	}
	if len(parsed.Unit) != 0 {
		ingredient.Unit = &domain.Unit{Name: parsed.Unit}
	}
}

func (s *ScraperService) kripToRecipe(kripRecipe *model.Recipe) *domain.Recipe {
	recipe := &domain.Recipe{}
	recipe.IsBasedOn = &kripRecipe.Url
	if len(kripRecipe.Name) > 0 {
		recipe.Name = &kripRecipe.Name
	}
	if len(kripRecipe.Description) > 0 {
		recipe.Description = &kripRecipe.Description
	}
	if len(kripRecipe.Language) > 0 {
		recipe.Language = &kripRecipe.Language
	}
	for _, image := range kripRecipe.Images {
		recipe.Images = append(recipe.Images, s.kripToImage(image))
	}
	if kripRecipe.Author != nil {
		recipe.Author = s.kripToAuthor(kripRecipe.Author)
	}
	if len(kripRecipe.Text) > 0 {
		recipe.Text = &kripRecipe.Text
	}
	if len(kripRecipe.PrepTime) != 0 {
		if d, err := types.DurationFromISO8601(kripRecipe.PrepTime); err == nil {
			recipe.PrepTime = &d
		}
	}
	if len(kripRecipe.CookTime) != 0 {
		if d, err := types.DurationFromISO8601(kripRecipe.CookTime); err == nil {
			recipe.CookTime = &d
		}
	}
	if len(kripRecipe.TotalTime) != 0 {
		if d, err := types.DurationFromISO8601(kripRecipe.TotalTime); err == nil {
			recipe.TotalTime = &d
		}
	}
	if len(kripRecipe.Difficulty) > 0 {
		recipe.Difficulty = &kripRecipe.Difficulty
	}
	if len(kripRecipe.CookingMethod) > 0 {
		recipe.Method = &kripRecipe.CookingMethod
	}
	for _, diet := range kripRecipe.Diets {
		recipe.Taxonomies = append(recipe.Taxonomies, &domain.Taxonomy{Type: "diet", Label: diet, Slug: utils.CreateTag(diet)})
	}
	for _, cat := range kripRecipe.Categories {
		recipe.Taxonomies = append(recipe.Taxonomies, &domain.Taxonomy{Type: "category", Label: cat, Slug: utils.CreateTag(cat)})
	}
	for _, cuisine := range kripRecipe.Cuisines {
		recipe.Taxonomies = append(recipe.Taxonomies, &domain.Taxonomy{Type: "cuisine", Label: cuisine, Slug: utils.CreateTag(cuisine)})
	}
	for _, keyword := range kripRecipe.Keywords {
		recipe.Taxonomies = append(recipe.Taxonomies, &domain.Taxonomy{Type: "keyword", Label: keyword, Slug: utils.CreateTag(keyword)})
	}
	if len(kripRecipe.Yield) != 0 {
		if yield := kUtils.FindInt(&kripRecipe.Yield); yield > 0 {
			recipe.Yield = &yield
		}
	}
	if kripRecipe.Nutrition != nil {
		recipe.Nutrition = &domain.Nutrition{
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
		}
	}
	if kripRecipe.Rating != nil {
		recipe.Rating = &domain.Rating{
			Reviews: kripRecipe.Rating.ReviewCount,
			Count:   kripRecipe.Rating.RatingCount,
			Value:   kripRecipe.Rating.RatingValue,
		}
	}
	if kripRecipe.Video != nil {
		recipe.Video = &domain.Video{
			Name:         kripRecipe.Video.Name,
			Description:  kripRecipe.Video.Description,
			EmbedUrl:     kripRecipe.Video.EmbedUrl,
			ContentUrl:   kripRecipe.Video.ContentUrl,
			ThumbnailUrl: kripRecipe.Video.ThumbnailUrl,
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
	if len(kripRecipe.Equipment) > 0 {
		equipment := make([]string, 0, len(kripRecipe.Equipment))
		for _, eq := range kripRecipe.Equipment {
			equipment = append(equipment, eq.Name)
		}
		recipe.Equipment = &equipment
	}
	for _, item := range kripRecipe.Instructions {
		recipe.Instructions = append(recipe.Instructions, s.kripToInstruction(&item.HowToStep))
		for _, step := range item.Steps {
			recipe.Instructions = append(recipe.Instructions, s.kripToInstruction(step))
		}
	}
	return recipe
}

func (s *ScraperService) kripToAuthor(person *krip.Person) *domain.Author {
	return &domain.Author{
		Name:        person.Name,
		Description: person.Description,
		Url:         person.Url,
		Image:       person.Image,
	}
}

func (s *ScraperService) kripToImage(image *krip.ImageObject) *domain.RecipeImage {
	return &domain.RecipeImage{
		RemoteUrl: image.Url,
		Width:     image.Width,
		Height:    image.Height,
		Caption:   image.Caption,
	}
}

func (s *ScraperService) kripToInstruction(item *krip.HowToStep) *domain.RecipeInstruction {
	ins := &domain.RecipeInstruction{}
	if len(item.Name) != 0 {
		ins.Title = &item.Name
	}
	if len(item.Text) != 0 {
		ins.Text = item.Text
	}
	if len(item.Url) != 0 {
		ins.Url = &item.Url
	}
	if len(item.Image) != 0 {
		ins.Image = &item.Image
	}
	if len(item.Video) != 0 {
		ins.Video = &item.Video
	}
	return ins
}

func (s *ScraperService) kripToIngredient(item *krip.PropertyValue) *domain.RecipeIngredient {
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
			ing.Food = &domain.Food{Name: item.Name}
		}
	} else if item.UnitText != "" {
		// Partially: unit and name are known, quantity is not.
		ing.RawText = item.UnitText + " " + item.Name
		ing.Unit = &domain.Unit{Name: item.UnitText}
		if item.Name != "" {
			ing.Food = &domain.Food{Name: item.Name}
		}
	} else {
		// Unstructured: Name holds the full ingredient string (e.g. "2 cups flour").
		ing.RawText = item.Name
	}
	return ing
}

func (s *ScraperService) kripToPublisher(org *krip.Organization) *domain.Publisher {
	pub := &domain.Publisher{Name: org.Name}
	if len(org.Description) != 0 {
		pub.Description = &org.Description
	}
	if len(org.Url) != 0 {
		pub.Url = org.Url
	}
	if len(org.Logo) != 0 {
		pub.RemoteImage = &org.Logo
	}
	return pub
}
