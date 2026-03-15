package services

import (
	"borscht.app/smetana/domain"
)

type taxonomyService struct {
	repo domain.TaxonomyRepository
}

func NewTaxonomyService(repo domain.TaxonomyRepository) domain.TaxonomyService {
	return &taxonomyService{repo: repo}
}

func (s *taxonomyService) List(taxonomyType string, offset, limit int) ([]domain.Taxonomy, int64, error) {
	return s.repo.List(taxonomyType, offset, limit)
}
