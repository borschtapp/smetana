package services

import (
	"context"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type equipmentService struct {
	repo         domain.EquipmentRepository
	imageService domain.ImageService
}

func NewEquipmentService(repo domain.EquipmentRepository, imageService domain.ImageService) domain.EquipmentService {
	return &equipmentService{repo: repo, imageService: imageService}
}

func (s *equipmentService) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Equipment, int64, error) {
	return s.repo.Search(householdID, opts)
}

func (s *equipmentService) FindOrCreate(ctx context.Context, equipment *domain.Equipment) error {
	if err := s.repo.FindOrCreate(equipment); err != nil {
		return err
	}

	if equipment != nil && equipment.ImagePath == nil && len(equipment.Images) > 0 {
		path, err := s.imageService.PersistRemoteAsDefault(ctx, equipment.Images[0], "equipments", equipment.ID, "")
		if err != nil {
			log.Warnw("unable to process equipment image, skipping", "equipment_id", equipment.ID, "image", equipment.Images[0], "error", err.Error())
		}
		equipment.ImagePath = path
	}
	return nil
}
