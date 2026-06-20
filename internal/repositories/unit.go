package repositories

import (
	"fmt"

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

func (r *unitRepository) Update(unit *domain.Unit) error {
	if err := r.db.Model(unit).Select("name").Updates(unit).Error; err != nil {
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
