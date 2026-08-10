package database

import (
	"borscht.app/smetana/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedUnits inserts standard base units and common derived units.
func SeedUnits(db *gorm.DB) error {
	bases := []domain.Unit{
		{Slug: "g", Name: "gram", Imperial: false, BaseFactor: 1},
		{Slug: "ml", Name: "milliliter", Imperial: false, BaseFactor: 1},
		{Slug: "cm", Name: "centimeter", Imperial: false, BaseFactor: 1},

		{Slug: "can", Name: "can", Imperial: false},
		{Slug: "pc", Name: "piece", Imperial: false},
		{Slug: "pkg", Name: "package", Imperial: false},
		{Slug: "bunch", Name: "bunch", Imperial: false},
		{Slug: "pinch", Name: "pinch", Imperial: false},
		{Slug: "slice", Name: "slice", Imperial: false},
		{Slug: "sprig", Name: "sprig", Imperial: false},
		{Slug: "stick", Name: "stick", Imperial: false},
		{Slug: "clove", Name: "clove", Imperial: false},
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "slug"}},
		DoUpdates: clause.AssignmentColumns([]string{"imperial", "base_unit_id", "base_factor", "updated"}),
	}).Create(&bases).Error; err != nil {
		return err
	}

	// Re-fetch base IDs in case rows existed before this run.
	var existing []domain.Unit
	if err := db.Select("id, slug").Where("slug IN ?", []string{"g", "ml", "cm", "pc"}).Find(&existing).Error; err != nil {
		return err
	}
	idx := make(map[string]uuid.UUID, len(existing))
	for _, u := range existing {
		idx[u.Slug] = u.ID
	}

	gramID := new(idx["g"])
	mliterID := new(idx["ml"])
	cmeterID := new(idx["cm"])

	units := []domain.Unit{
		{Slug: "mg", Name: "milligram", BaseUnitID: gramID, BaseFactor: 0.001},
		{Slug: "kg", Name: "kilogram", BaseUnitID: gramID, BaseFactor: 1000},
		{Slug: "t", Name: "metric ton", BaseUnitID: gramID, BaseFactor: 1_000_000},
		{Slug: "oz", Name: "ounce", BaseUnitID: gramID, BaseFactor: 28, Imperial: true},
		{Slug: "lb", Name: "pound", BaseUnitID: gramID, BaseFactor: 454, Imperial: true},
		{Slug: "st", Name: "stone", BaseUnitID: gramID, BaseFactor: 6350, Imperial: true},

		{Slug: "l", Name: "liter", BaseUnitID: mliterID, BaseFactor: 1000},
		{Slug: "dl", Name: "deciliter", BaseUnitID: mliterID, BaseFactor: 100},
		{Slug: "cl", Name: "centiliter", BaseUnitID: mliterID, BaseFactor: 10},
		// I have discovered that imperial tsp is 18% larger than tsp, it was difficult to fall asleep with this.
		// I decided, I don't care, it's anyway easier to read when we have 5 ml and not 4.928922 or 5.91939 ml 🙄
		{Slug: "tsp", Name: "teaspoon", BaseUnitID: mliterID, BaseFactor: 5, Imperial: true},
		{Slug: "tbsp", Name: "tablespoon", BaseUnitID: mliterID, BaseFactor: 15, Imperial: true},
		{Slug: "fl-oz", Name: "fluid ounce", BaseUnitID: mliterID, BaseFactor: 30, Imperial: true},
		{Slug: "cup", Name: "cup", BaseUnitID: mliterID, BaseFactor: 240, Imperial: true},
		{Slug: "pt", Name: "pint", BaseUnitID: mliterID, BaseFactor: 480, Imperial: true},
		{Slug: "qt", Name: "quart", BaseUnitID: mliterID, BaseFactor: 950, Imperial: true},
		{Slug: "gal", Name: "gallon", BaseUnitID: mliterID, BaseFactor: 3800, Imperial: true},

		{Slug: "mm", Name: "millimeter", BaseUnitID: cmeterID, BaseFactor: 0.1},
		{Slug: "dm", Name: "decimeter", BaseUnitID: cmeterID, BaseFactor: 10},
		{Slug: "m", Name: "meter", BaseUnitID: cmeterID, BaseFactor: 100},
		{Slug: "in", Name: "inch", BaseUnitID: cmeterID, BaseFactor: 25, Imperial: true},
		{Slug: "ft", Name: "foot", BaseUnitID: cmeterID, BaseFactor: 300, Imperial: true},
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "slug"}},
		DoUpdates: clause.AssignmentColumns([]string{"imperial", "base_unit_id", "base_factor", "updated"}),
	}).Create(&units).Error; err != nil {
		return err
	}

	return nil
}
