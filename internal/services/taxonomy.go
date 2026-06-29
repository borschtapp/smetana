package services

import (
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type taxonomyService struct {
	repo domain.TaxonomyRepository
}

func NewTaxonomyService(repo domain.TaxonomyRepository) domain.TaxonomyService {
	return &taxonomyService{repo: repo}
}

func (s *taxonomyService) ByID(id uuid.UUID) (*domain.Taxonomy, error) {
	return s.repo.ByID(id)
}

func (s *taxonomyService) ByIDs(ids []uuid.UUID) (map[uuid.UUID]*domain.Taxonomy, error) {
	return s.repo.ByIDs(ids)
}

func (s *taxonomyService) Search(taxonomyType string, householdID uuid.UUID, opts types.SearchOptions) ([]domain.Taxonomy, int64, error) {
	return s.repo.Search(taxonomyType, householdID, opts)
}

func (s *taxonomyService) FindOrCreate(taxonomy *domain.Taxonomy) error {
	return s.repo.FindOrCreate(taxonomy)
}

func (s *taxonomyService) Merge(keepID, mergeID uuid.UUID) error {
	return s.repo.Merge(keepID, mergeID)
}

func (s *taxonomyService) Detach(id uuid.UUID) error {
	return s.repo.Detach(id)
}
