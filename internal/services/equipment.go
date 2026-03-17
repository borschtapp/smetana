package services

import (
	"context"

	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/domain"
)

type equipmentService struct {
	repo         domain.EquipmentRepository
	imageService domain.ImageService
}

func NewEquipmentService(repo domain.EquipmentRepository, imageService domain.ImageService) domain.EquipmentService {
	return &equipmentService{repo: repo, imageService: imageService}
}

func (s *equipmentService) Search(query string, offset, limit int) ([]domain.Equipment, int64, error) {
	return s.repo.Search(query, offset, limit)
}

func (s *equipmentService) FindOrCreate(ctx context.Context, equipment *domain.Equipment) error {
	if err := s.repo.FindOrCreate(equipment); err != nil {
		return err
	}

	if equipment != nil && equipment.ImagePath == nil && len(equipment.Images) > 0 {
		path, err := s.imageService.PersistRemoteAsDefault(ctx, equipment.Images[0], "equipments", equipment.ID, "")
		if err != nil {
			log.Warnw("unable to process equipment image, skipping", "equipment_id", equipment.ID, "image", equipment.Images[0], "error", err)
		}
		equipment.ImagePath = path
	}
	return nil
}
