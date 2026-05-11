package services

import (
	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	kUtils "github.com/borschtapp/krip/utils"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type scraperMapper struct {
	parser IngredientParser
}

func newScraperMapper(parser IngredientParser) *scraperMapper {
	return &scraperMapper{parser: parser}
}

func (m *scraperMapper) toRecipe(kripRecipe *krip.Recipe) *domain.Recipe {
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
			recipe.Images = append(recipe.Images, m.toImage(image))
		}
	}
	if kripRecipe.Author != nil && kripRecipe.Author.Name != "" {
		recipe.Author = m.toAuthor(kripRecipe.Author)
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
	m.addTaxonomies(recipe, domain.TaxonomyTypeDiet, kripRecipe.Diets)
	m.addTaxonomies(recipe, domain.TaxonomyTypeCategory, kripRecipe.Categories)
	m.addTaxonomies(recipe, domain.TaxonomyTypeCuisine, kripRecipe.Cuisines)
	m.addTaxonomies(recipe, domain.TaxonomyTypeKeyword, kripRecipe.Keywords)
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
		recipe.Publisher = m.toPublisher(kripRecipe.Publisher)
	}
	if kripRecipe.DateModified != nil {
		recipe.Published = kripRecipe.DateModified
	} else if kripRecipe.DatePublished != nil {
		recipe.Published = kripRecipe.DatePublished
	}
	for _, item := range kripRecipe.Ingredients {
		recipe.Ingredients = append(recipe.Ingredients, m.toIngredient(item))
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
		if item.Text != "" || item.Name != "" {
			recipe.Instructions = append(recipe.Instructions, m.toInstruction(&item.HowToStep))
		}
		for _, step := range item.Steps {
			recipe.Instructions = append(recipe.Instructions, m.toInstruction(step))
		}
	}
	m.enrichIngredients(recipe.Ingredients, kripRecipe.Language)
	return recipe
}

func (m *scraperMapper) addTaxonomies(recipe *domain.Recipe, taxType string, labels []string) {
	for _, label := range labels {
		recipe.Taxonomies = append(recipe.Taxonomies, &domain.Taxonomy{
			Type:  taxType,
			Label: label,
			Slug:  utils.CreateTag(label),
		})
	}
}

func (m *scraperMapper) toAuthor(person *krip.Person) *domain.Author {
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

func (m *scraperMapper) toImage(image *krip.ImageObject) *domain.Image {
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

func (m *scraperMapper) toInstruction(item *krip.HowToStep) *domain.RecipeInstruction {
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

func (m *scraperMapper) toIngredient(item *krip.PropertyValue) *domain.RecipeIngredient {
	ing := &domain.RecipeIngredient{}
	if item.Value != "" {
		if item.UnitText != "" {
			ing.RawText = item.Value + " " + item.UnitText + " " + item.Name
		} else {
			ing.RawText = item.Value + " " + item.Name
		}

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
		ing.RawText = item.UnitText + " " + item.Name
		ing.Unit = &domain.Unit{Name: item.UnitText}
		if item.Name != "" {
			ing.Name = &item.Name
			ing.Food = &domain.Food{Name: item.Name, Slug: utils.CreateTag(item.Name)}
		}
	} else {
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

func (m *scraperMapper) toFeed(f *krip.Feed) *domain.Feed {
	feed := &domain.Feed{
		Url:        utils.NormalizeURL(f.Url),
		Name:       f.Name,
		Discovered: f.Discovered,
	}
	if f.Description != "" {
		feed.Description = &f.Description
	}
	if f.Publisher != nil {
		feed.Publisher = m.toPublisher(f.Publisher)
	}
	return feed
}

func (m *scraperMapper) toPublisher(org *krip.Organization) *domain.Publisher {
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

func (m *scraperMapper) enrichIngredients(ingredients []*domain.RecipeIngredient, language string) {
	for _, ingredient := range ingredients {
		m.enrichIngredient(ingredient, language)
	}
}

func (m *scraperMapper) enrichIngredient(ingredient *domain.RecipeIngredient, language string) {
	if ingredient.Food != nil {
		return
	}

	parsed, err := m.parser.ParseIngredient(ingredient.RawText, kapusta.IngredientOptions{Lang: language})
	if err != nil {
		return
	}

	if parsed.Amount != 0 {
		ingredient.Amount = &parsed.Amount
	}
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
