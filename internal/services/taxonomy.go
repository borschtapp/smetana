package services

import (
	"borscht.app/smetana/domain"
)

type TaxonomyService struct {
	repo domain.TaxonomyRepository
}

func NewTaxonomyService(repo domain.TaxonomyRepository) domain.TaxonomyService {
	return &TaxonomyService{repo: repo}
}

func (s *TaxonomyService) List(taxonomyType string, offset, limit int) ([]domain.Taxonomy, int64, error) {
	return s.repo.List(taxonomyType, offset, limit)
}
