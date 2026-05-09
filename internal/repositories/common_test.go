package repositories_test

// Junction table structs for GORM .Model() calls in tests
// These are defined here to avoid duplication across multiple test files in the same package.

type recipeEquipment struct{}

func (recipeEquipment) TableName() string {
	return "recipe_equipment"
}

type recipeTaxonomy struct{}

func (recipeTaxonomy) TableName() string {
	return "recipe_taxonomies"
}

type feedSubscription struct{}

func (feedSubscription) TableName() string {
	return "feed_subscriptions"
}

type collectionRecipe struct{}

func (collectionRecipe) TableName() string {
	return "collection_recipes"
}
