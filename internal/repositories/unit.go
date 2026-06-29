package repositories

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/utils"
)

type unitRepository struct {
	db *gorm.DB
}

func NewUnitRepository(db *gorm.DB) domain.UnitRepository {
	return &unitRepository{db: db}
}

func (r *unitRepository) ByID(id uuid.UUID) (*domain.Unit, error) {
	var unit domain.Unit
	if err := r.db.First(&unit, id).Error; err != nil {
		return nil, fmt.Errorf("unit by id %s: %w", id, mapErr(err))
	}
	return &unit, nil
}

func (r *unitRepository) ByIDs(ids []uuid.UUID) (map[uuid.UUID]*domain.Unit, error) {
	return byIDs(r.db, ids, func(u *domain.Unit) uuid.UUID { return u.ID })
}

func (r *unitRepository) FindOrCreate(unit *domain.Unit) error {
	if unit.Slug == "" {
		unit.Slug = utils.CreateTag(unit.Name)
	}

	if err := r.db.First(unit, "slug = ?", unit.Slug).Error; err == nil {
		return r.resolveAlias(unit)
	}

	if err := r.db.First(unit, "lower(name) = lower(?)", unit.Name).Error; err == nil {
		return r.resolveAlias(unit)
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(unit)
	if result.Error != nil {
		return fmt.Errorf("create unit: %w", mapErr(result.Error))
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		if err := r.db.First(unit, "slug = ?", unit.Slug).Error; err != nil {
			return fmt.Errorf("find unit after conflict: %w", mapErr(err))
		}
		return r.resolveAlias(unit)
	}

	return nil
}

// resolveAlias replaces unit with its canonical target when the row is an alias.
func (r *unitRepository) resolveAlias(unit *domain.Unit) error {
	if unit.CanonicalUnitID == nil {
		return nil
	}
	var canonical domain.Unit
	if err := r.db.First(&canonical, *unit.CanonicalUnitID).Error; err != nil {
		return fmt.Errorf("resolve unit alias: %w", mapErr(err))
	}
	*unit = canonical
	return nil
}

func (r *unitRepository) Search(query string, imperial *bool, offset, limit int) ([]domain.Unit, int64, error) {
	db := r.db.Model(&domain.Unit{}).Scopes(IsCanonicalUnit)
	if query != "" {
		db = db.Where("name LIKE ? OR slug LIKE ?", "%"+query+"%", "%"+query+"%")
	}
	if imperial != nil {
		db = db.Where("imperial = ?", *imperial)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("search count units: %w", mapErr(err))
	}

	var units []domain.Unit
	if err := db.Offset(offset).Limit(limit).Find(&units).Error; err != nil {
		return nil, 0, fmt.Errorf("search find units: %w", mapErr(err))
	}
	return units, total, nil
}

func (r *unitRepository) Merge(keepID, mergeID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var keep, merge domain.Unit
		if err := tx.Scopes(IsCanonicalUnit).First(&keep, keepID).Error; err != nil {
			return fmt.Errorf("keep unit: %w", mapErr(err))
		}
		if err := tx.Scopes(IsCanonicalUnit).First(&merge, mergeID).Error; err != nil {
			return fmt.Errorf("merge unit: %w", mapErr(err))
		}

		if err := tx.Model(&domain.RecipeIngredient{}).Where("unit_id = ?", mergeID).Update("unit_id", keepID).Error; err != nil {
			return fmt.Errorf("reassign ingredients: %w", mapErr(err))
		}
		if err := tx.Model(&domain.ShoppingItem{}).Where("unit_id = ?", mergeID).Update("unit_id", keepID).Error; err != nil {
			return fmt.Errorf("reassign shopping items: %w", mapErr(err))
		}
		if err := tx.Model(&domain.FoodPrice{}).Where("unit_id = ?", mergeID).Update("unit_id", keepID).Error; err != nil {
			return fmt.Errorf("reassign food prices: %w", mapErr(err))
		}
		if err := tx.Model(&domain.Food{}).Where("default_unit_id = ?", mergeID).Update("default_unit_id", keepID).Error; err != nil {
			return fmt.Errorf("reassign food default units: %w", mapErr(err))
		}
		// Re-parent derived units that used mergeID as their base, excluding keepID itself.
		if err := tx.Model(&domain.Unit{}).Where("base_unit_id = ? AND id != ?", mergeID, keepID).Update("base_unit_id", keepID).Error; err != nil {
			return fmt.Errorf("reassign derived units: %w", mapErr(err))
		}

		// Preserve fields from the discarded unit if the kept unit lacks them.
		inherited := map[string]any{}
		if keep.BaseUnitID == nil && merge.BaseUnitID != nil {
			inherited["base_unit_id"] = merge.BaseUnitID
		}
		if keep.BaseFactor == 0 && merge.BaseFactor != 0 {
			inherited["base_factor"] = merge.BaseFactor
		}
		if !keep.Imperial && merge.Imperial {
			inherited["imperial"] = true
		}
		if len(inherited) > 0 {
			if err := tx.Model(&domain.Unit{}).Where("id = ?", keepID).Updates(inherited).Error; err != nil {
				return fmt.Errorf("propagate fields to keep unit: %w", mapErr(err))
			}
		}

		if err := tx.Model(&domain.Unit{}).Where("id = ?", mergeID).Update("canonical_unit_id", keepID).Error; err != nil {
			return fmt.Errorf("set unit alias: %w", mapErr(err))
		}
		return nil
	})
}

func (r *unitRepository) Detach(id uuid.UUID) error {
	return detach[domain.Unit](r.db, id, "canonical_unit_id")
}

func (r *unitRepository) Update(unit *domain.Unit) error {
	cols := []string{"name", "slug", "base_unit_id", "base_factor", "imperial"}
	if err := r.db.Model(unit).Select(cols).Updates(unit).Error; err != nil {
		return fmt.Errorf("update unit %s: %w", unit.ID, mapErr(err))
	}
	return nil
}

func (r *unitRepository) ByBase(baseUnitID uuid.UUID, imperial bool) ([]domain.Unit, error) {
	var units []domain.Unit
	if err := r.db.Where("(id = ? OR base_unit_id = ?) AND imperial = ?", baseUnitID, baseUnitID, imperial).Find(&units).Error; err != nil {
		return nil, fmt.Errorf("find units by base %s: %w", baseUnitID, mapErr(err))
	}
	return units, nil
}

func (r *unitRepository) AddTaxonomy(unitID uuid.UUID, taxonomy *domain.Taxonomy) error {
	if err := r.db.Model(&domain.Unit{ID: unitID}).Association("Taxonomies").Append(taxonomy); err != nil {
		return fmt.Errorf("add taxonomy %s to unit %s: %w", taxonomy.ID, unitID, mapErr(err))
	}
	return nil
}

// The "no conversion yet" check is done as a second query rather than a correlated
// NOT EXISTS subquery, to keep both queries expressible with the plain query builder.
func (r *unitRepository) NonConvertibleUsageSince(since time.Time) ([]domain.FoodUnitPair, error) {
	var candidates []domain.FoodUnitPair
	err := r.db.Model(&domain.RecipeIngredient{}).
		Distinct("recipe_ingredients.food_id AS food_id, recipe_ingredients.unit_id AS unit_id").
		Joins("JOIN food ON food.id = recipe_ingredients.food_id").
		Joins("JOIN units ON units.id = recipe_ingredients.unit_id").
		Where("recipe_ingredients.food_id IS NOT NULL AND recipe_ingredients.unit_id IS NOT NULL").
		Where("food.canonical_food_id IS NULL AND food.updated > ?", since).
		Scopes(NonConvertibleUnit).
		Scan(&candidates).Error
	if err != nil {
		return nil, fmt.Errorf("non-convertible usage since %s: %w", since, mapErr(err))
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	foodIDs := make([]uuid.UUID, len(candidates))
	for i, c := range candidates {
		foodIDs[i] = c.FoodID
	}
	var existing []domain.FoodUnitConversion
	if err := r.db.Where("food_id IN ?", foodIDs).Find(&existing).Error; err != nil {
		return nil, fmt.Errorf("existing conversions for non-convertible usage: %w", mapErr(err))
	}
	hasConversion := make(map[domain.FoodUnitPair]bool, len(existing))
	for _, c := range existing {
		hasConversion[domain.FoodUnitPair{FoodID: c.FoodID, UnitID: c.UnitID}] = true
	}

	result := candidates[:0]
	for _, c := range candidates {
		if !hasConversion[c] {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *unitRepository) MergeCandidates(since time.Time) (workSet, catalog []domain.Unit, err error) {
	if err := r.db.Scopes(IsCanonicalUnit).Where("updated > ?", since).Find(&workSet).Error; err != nil {
		return nil, nil, fmt.Errorf("merge candidates workset: %w", mapErr(err))
	}
	if err := r.db.Scopes(IsCanonicalUnit).Find(&catalog).Error; err != nil {
		return nil, nil, fmt.Errorf("merge candidates catalog: %w", mapErr(err))
	}
	return workSet, catalog, nil
}
