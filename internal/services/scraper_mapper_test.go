package services

import (
	"testing"

	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"borscht.app/smetana/domain"
)

func TestScraperMapper_ToRecipe(t *testing.T) {
	provider := &KripProvider{}
	mapper := newScraperMapper(provider)

	kRecipe := &krip.Recipe{
		Url:  "https://example.com/recipe",
		Name: "Test Recipe",
		Ingredients: []*krip.PropertyValue{
			{Name: "flour", Value: "2", UnitText: "cups"},
			{Name: "egg", Value: "1"},
		},
		Instructions: []*krip.HowToSection{
			{
				HowToStep: krip.HowToStep{Name: "Step 1", Text: "Mix it"},
			},
		},
	}

	recipe := mapper.toRecipe(kRecipe)

	assert.NotNil(t, recipe)
	assert.Equal(t, "https://example.com/recipe", *recipe.SourceUrl)
	assert.Equal(t, "Test Recipe", *recipe.Name)
	assert.Len(t, recipe.Ingredients, 2)
	assert.Equal(t, "2 cups flour", recipe.Ingredients[0].RawText)
	assert.Equal(t, "1 egg", recipe.Ingredients[1].RawText)
	assert.Len(t, recipe.Instructions, 1)
	assert.Equal(t, "Step 1", *recipe.Instructions[0].Title)
	assert.Equal(t, "Mix it", recipe.Instructions[0].Text)
}

func TestScraperMapper_EnrichIngredient(t *testing.T) {
	mockProvider := &mockScraperProvider{}
	parsed := kapusta.Ingredient{Amount: 200, Unit: "g", Name: "sugar"}
	mockProvider.On("ParseIngredient", "200g sugar", mock.Anything).Return(parsed, nil)
	mapper := newScraperMapper(mockProvider)

	ing := &domain.RecipeIngredient{RawText: "200g sugar"}
	mapper.enrichIngredient(ing, "en")

	assert.NotNil(t, ing.Amount)
	assert.Equal(t, float64(200), *ing.Amount)
	assert.NotNil(t, ing.Unit)
	assert.Equal(t, "g", ing.Unit.Name)
	assert.NotNil(t, ing.Name)
	assert.Equal(t, "sugar", *ing.Name)
	mockProvider.AssertExpectations(t)
}
