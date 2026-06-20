package services

import (
	"context"
	"fmt"
	"regexp"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

var proxyImageRe = regexp.MustCompile(`!\[\]\(proxy:([^)]+)\)`)

type recipeIngestService struct {
	recipeService    domain.RecipeService
	imageService     domain.ImageService
	foodService      domain.FoodService
	unitService      domain.UnitService
	publisherService domain.PublisherService
	authorService    domain.AuthorService
	taxonomyService  domain.TaxonomyService
	equipmentService domain.EquipmentService
	fetchConcurrency int
}

func NewRecipeIngestService(
	recipeService domain.RecipeService,
	imageService domain.ImageService,
	foodService domain.FoodService,
	unitService domain.UnitService,
	publisherService domain.PublisherService,
	authorService domain.AuthorService,
	taxonomyService domain.TaxonomyService,
	equipmentService domain.EquipmentService,
) domain.RecipeIngestService {
	return &recipeIngestService{
		recipeService:    recipeService,
		imageService:     imageService,
		foodService:      foodService,
		unitService:      unitService,
		publisherService: publisherService,
		authorService:    authorService,
		taxonomyService:  taxonomyService,
		equipmentService: equipmentService,
		fetchConcurrency: utils.GetenvInt("FETCH_CONCURRENCY", 5),
	}
}

func (s *recipeIngestService) ImportRecipe(ctx context.Context, recipe *domain.Recipe) (*domain.Recipe, error) {
	if recipe == nil {
		return nil, sentinels.BadRequest("recipe is nil")
	}

	if recipe.ID == uuid.Nil {
		recipe.ID, _ = uuid.NewV7()
	}

	for _, inst := range recipe.Instructions {
		if inst.ID == uuid.Nil {
			inst.ID, _ = uuid.NewV7()
		}
	}

	// 1. Persist main images
	s.processRecipeImages(ctx, recipe)

	g, resCtx := errgroup.WithContext(ctx)
	g.SetLimit(s.fetchConcurrency)

	// 2. Resolve publisher
	if recipe.Publisher != nil {
		g.Go(func() error {
			if err := s.publisherService.FindOrCreate(resCtx, recipe.Publisher); err != nil {
				log.Warnw("failed to resolve publisher", "publisher", recipe.Publisher, "error", err.Error())
			} else {
				recipe.PublisherID = new(recipe.Publisher.ID)
			}
			return nil
		})
	}

	// 3. Resolve author
	if recipe.Author != nil {
		g.Go(func() error {
			if err := s.authorService.FindOrCreate(resCtx, recipe.Author); err != nil {
				log.Warnw("failed to resolve author", "author", recipe.Author, "error", err.Error())
			} else {
				recipe.AuthorID = new(recipe.Author.ID)
			}
			return nil
		})
	}

	// 4. Resolve taxonomies
	for _, t := range recipe.Taxonomies {
		t := t
		g.Go(func() error {
			if err := s.taxonomyService.FindOrCreate(t); err != nil {
				log.Warnw("failed to resolve taxonomy", "taxonomy", t, "error", err.Error())
			}
			return nil
		})
	}

	// 5. Resolve equipment
	for _, e := range recipe.Equipment {
		e := e
		g.Go(func() error {
			if err := s.equipmentService.FindOrCreate(resCtx, e); err != nil {
				log.Warnw("failed to resolve equipment", "equipment", e, "error", err.Error())
			}
			return nil
		})
	}

	// 6. Resolve food and units for ingredients
	uniqueFoods := make(map[string]*domain.Food)
	for _, ing := range recipe.Ingredients {
		if ing.Food != nil {
			uniqueFoods[ing.Food.Slug] = ing.Food
		}
	}

	for _, food := range uniqueFoods {
		food := food
		g.Go(func() error {
			if err := s.foodService.FindOrCreate(resCtx, food); err != nil {
				log.Warnw("error creating food", "food", food.Name, "error", err.Error())
			} else {
				s.resolveFoodTaxonomies(food)
			}
			return nil
		})
	}

	for _, ing := range recipe.Ingredients {
		ing := ing
		g.Go(func() error {
			if ing.Unit != nil {
				if err := s.unitService.FindOrCreate(ing.Unit); err != nil {
					log.Warnw("error creating unit", "unit", ing.Unit, "error", err.Error())
					ing.Unit = nil
				} else {
					ing.UnitID = new(ing.Unit.ID)
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("recipe resolution completed with errors", "recipeID", recipe.ID, "error", err.Error())
	}

	// Link resolved foods to ingredients
	for _, ing := range recipe.Ingredients {
		if ing.Food != nil {
			if food, ok := uniqueFoods[ing.Food.Slug]; ok && food.ID != uuid.Nil {
				ing.FoodID = new(food.ID)
			} else {
				ing.Food = nil
			}
		}
	}

	// drop taxonomies/equipment that failed to resolve, i.e. still have a nil ID:
	var validTaxonomies []*domain.Taxonomy
	for _, t := range recipe.Taxonomies {
		if t.ID != uuid.Nil {
			validTaxonomies = append(validTaxonomies, t)
		}
	}
	recipe.Taxonomies = validTaxonomies

	var validEquipment []*domain.Equipment
	for _, e := range recipe.Equipment {
		if e.ID != uuid.Nil {
			validEquipment = append(validEquipment, e)
		}
	}
	recipe.Equipment = validEquipment

	// 7. Persist instruction images
	s.processInstructionImages(ctx, recipe)

	// 8. Save global recipe
	if err := s.recipeService.Import(recipe); err != nil {
		return nil, err
	}

	return recipe, nil
}

func (s *recipeIngestService) processRecipeImages(ctx context.Context, recipe *domain.Recipe) {
	if len(recipe.Images) == 0 {
		return
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(s.fetchConcurrency)

	for _, img := range recipe.Images {
		if img.SourceURL == "" || img.ID != uuid.Nil {
			continue // skip empty or already persisted
		}
		img := img
		g.Go(func() error {
			img.EntityType = "recipes"
			img.EntityID = recipe.ID
			return s.imageService.PersistRemote(ctx, img, "recipes")
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("recipe image processing completed with errors", "recipeID", recipe.ID, "error", err.Error())
	}

	// Select best image as default
	if best := s.selectBestImage(recipe.Images); best != nil {
		if err := s.imageService.SetDefault(best); err != nil {
			log.Warnw("failed to set default image", "imageID", best.ID, "error", err.Error())
		}
		recipe.ImagePath = best.Path
	}
}

func (s *recipeIngestService) processInstructionImages(ctx context.Context, recipe *domain.Recipe) {
	type imgRef struct {
		img  *domain.Image
		inst *domain.RecipeInstruction
	}
	var remoteRefs []imgRef
	for _, inst := range recipe.Instructions {
		for _, img := range inst.Images {
			if img.SourceURL != "" && img.ID == uuid.Nil {
				remoteRefs = append(remoteRefs, imgRef{img, inst})
			}
		}
	}

	if len(remoteRefs) == 0 {
		return
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(s.fetchConcurrency)

	for _, ref := range remoteRefs {
		g.Go(func() error {
			path, err := s.imageService.PersistRemoteAsDefault(ctx, ref.img, "recipe_instructions", ref.inst.ID, "recipes/"+recipe.ID.String())
			if err != nil {
				return err
			}
			ref.inst.ImagePath = path
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("instruction image processing completed with errors", "recipeID", recipe.ID, "error", err.Error())
	}

	// Update raw text if it contains image placeholders
	for _, inst := range recipe.Instructions {
		if inst.Text == "" {
			continue
		}
		// Build lookup for this instruction's images
		urlToID := make(map[string]uuid.UUID)
		for _, img := range inst.Images {
			if img.ID != uuid.Nil {
				urlToID[img.SourceURL] = img.ID
			}
		}

		inst.Text = proxyImageRe.ReplaceAllStringFunc(inst.Text, func(match string) string {
			groups := proxyImageRe.FindStringSubmatch(match)
			if len(groups) > 1 {
				url := groups[1]
				if id, ok := urlToID[url]; ok {
					return fmt.Sprintf("![](proxy:%s)", id)
				}
			}
			return match
		})
	}

}

func (s *recipeIngestService) selectBestImage(images []*domain.Image) *domain.Image {
	const maxPreferredDim = 1000
	var best, fallback *domain.Image
	var bestDim, fallbackDim int

	for _, image := range images {
		if image == nil || image.Path == nil || image.Width == nil || image.Height == nil {
			continue
		}
		dim := max(*image.Width, *image.Height)
		if dim <= maxPreferredDim && dim > bestDim {
			best = image
			bestDim = dim
		}
		if dim > fallbackDim {
			fallback = image
			fallbackDim = dim
		}
	}

	if best != nil {
		return best
	}
	return fallback
}

func (s *recipeIngestService) resolveFoodTaxonomies(food *domain.Food) {
	for _, t := range food.Taxonomies {
		if err := s.taxonomyService.FindOrCreate(t); err != nil {
			log.Warnw("failed to resolve food taxonomy", "taxonomy", t, "foodID", food.ID, "error", err.Error())
			continue
		}
		_ = s.foodService.AddTaxonomy(food.ID, t)
	}
}
