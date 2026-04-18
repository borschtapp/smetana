package repositories

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type equipmentRepository struct {
	db *gorm.DB
}

func NewEquipmentRepository(db *gorm.DB) domain.EquipmentRepository {
	return &equipmentRepository{db: db}
}

func (r *equipmentRepository) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Equipment, int64, error) {
	q := r.db.Model(&domain.Equipment{})
	if opts.SearchQuery != "" {
		q = q.Where("name LIKE ? OR slug LIKE ?", "%"+opts.SearchQuery+"%", "%"+opts.SearchQuery+"%")
	}

	if householdID != uuid.Nil && opts.Scope != "" {
		scopeWhere, scopeArgs := scopeWhereArgs(opts.Scope, householdID)
		q = q.Where(`EXISTS (
			SELECT 1 FROM recipe_equipment
			JOIN recipes ON recipes.id = recipe_equipment.recipe_id
			WHERE recipe_equipment.equipment_id = equipment.id
			AND `+scopeWhere+`
		)`, scopeArgs...)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, mapErr(err)
	} else if total == 0 {
		return nil, 0, nil
	}

	sortByRecipes := strings.EqualFold(opts.Sort, "total_recipes")

	selectCols := []string{"equipment.*"}
	var selectArgs []any

	if opts.Has("total_recipes") || sortByRecipes {
		if householdID == uuid.Nil {
			selectCols = append(selectCols, `(
				SELECT COUNT(*) FROM recipe_equipment
				WHERE recipe_equipment.equipment_id = equipment.id
			) AS total_recipes`)
		} else {
			scopeWhere, scopeArgs := scopeWhereArgs(opts.Scope, householdID)
			selectCols = append(selectCols, `(
				SELECT COUNT(*) FROM recipe_equipment
				JOIN recipes ON recipes.id = recipe_equipment.recipe_id
				WHERE recipe_equipment.equipment_id = equipment.id
				AND `+scopeWhere+`
			) AS total_recipes`)
			selectArgs = append(selectArgs, scopeArgs...)
		}
	}

	q = q.Select(strings.Join(selectCols, ", "), selectArgs...)

	if sortByRecipes {
		q = q.Order("total_recipes " + opts.Order)
	} else {
		q = q.Order(clause.OrderByColumn{
			Column: clause.Column{Table: "equipment", Name: opts.Sort},
			Desc:   strings.EqualFold(opts.Order, "DESC"),
		})
	}

	var equipment []domain.Equipment
	if err := q.Offset(opts.Offset).Limit(opts.Limit).Find(&equipment).Error; err != nil {
		return nil, 0, mapErr(err)
	}
	return equipment, total, nil
}

func (r *equipmentRepository) FindOrCreate(equipment *domain.Equipment) error {
	if equipment.Slug == "" {
		equipment.Slug = utils.CreateTag(equipment.Name)
	}

	if err := r.db.First(equipment, "slug = ?", equipment.Slug).Error; err == nil {
		return nil
	}

	if err := r.db.First(equipment, "lower(name) = lower(?)", equipment.Name).Error; err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(equipment)
	if result.Error != nil {
		return mapErr(result.Error)
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return mapErr(r.db.First(equipment, "slug = ?", equipment.Slug).Error)
	}

	return nil
}
