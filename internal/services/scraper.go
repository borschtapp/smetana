package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/borschtapp/krip"
	"github.com/doyensec/safeurl"
	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

type scraperService struct {
	provider ScraperProvider
	mapper   *scraperMapper
	timeout  time.Duration
}

func NewScraperService(provider ScraperProvider, parser IngredientParser) domain.ScraperService {
	return &scraperService{
		provider: provider,
		mapper:   newScraperMapper(parser),
		timeout:  utils.GetenvDuration("SCRAPE_TIMEOUT", 30*time.Second),
	}
}

func defaultRequestOptions(ctx context.Context) krip.RequestOptions {
	return krip.RequestOptions{
		Context:    ctx,
		HttpClient: safeurl.Client(safeurl.GetConfigBuilder().Build()),
	}
}

// scrapeErr maps a scrape timeout to 504 and wraps other errors with op. It lives here rather than in
// the error handler so other timeouts (DB, OIDC) still surface as logged 500s.
func scrapeErr(op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		log.Infow("scrape timed out", "op", op, "error", err.Error())
		return sentinels.GatewayTimeout("timed out fetching the source page")
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (s *scraperService) ScrapeUrl(ctx context.Context, url string, requestedType string) (*domain.ScrapeResult, error) {
	scrapeCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	options := krip.FeedOptions{
		ScrapeOptions: krip.ScrapeOptions{
			RequestOptions: defaultRequestOptions(scrapeCtx),
		},
	}

	data, err := s.provider.UrlInput(url, options.ScrapeOptions)
	if err != nil {
		return nil, scrapeErr("scrape url (fetch input)", err)
	}

	kripRecipe := &krip.Recipe{}
	var recipeErr error
	if requestedType == "" || requestedType == domain.ImportTypeAuto || requestedType == domain.ImportTypeRecipe {
		if recipeErr = s.provider.Scrape(data, kripRecipe, options.ScrapeOptions); recipeErr != nil {
			log.Infow("failed to scrape recipe", "url", url, "error", recipeErr.Error())
		}
	}

	if requestedType == domain.ImportTypeRecipe {
		if kripRecipe.IsValid() || kripRecipe.Name != "" {
			return &domain.ScrapeResult{Type: domain.PageTypeRecipe, Recipe: s.mapper.toRecipe(kripRecipe)}, nil
		}
		if recipeErr != nil {
			return nil, scrapeErr("scrape url (explicit recipe)", recipeErr)
		}
		return nil, sentinels.BadRequest("recipe: no valid recipe found at URL")
	}

	if requestedType == domain.ImportTypeFeed {
		kripFeed := &krip.Feed{}
		if err := s.provider.ScrapeFeed(data, kripFeed, options); err == nil && len(kripFeed.Entries) > 0 {
			return &domain.ScrapeResult{Type: domain.PageTypeFeed, Feed: s.mapper.toFeed(kripFeed)}, nil
		} else if err != nil {
			return nil, scrapeErr("scrape url (explicit feed)", err)
		}
		return nil, sentinels.BadRequest("feed: no valid feed found at URL")
	}

	if kripRecipe.IsValid() && len(kripRecipe.Ingredients) > 0 && len(kripRecipe.Instructions) > 0 {
		return &domain.ScrapeResult{Type: domain.PageTypeRecipe, Recipe: s.mapper.toRecipe(kripRecipe)}, nil
	}

	kripFeed := &krip.Feed{}
	if err := s.provider.ScrapeFeed(data, kripFeed, options); err == nil && len(kripFeed.Entries) > 0 {
		return &domain.ScrapeResult{Type: domain.PageTypeFeed, Feed: s.mapper.toFeed(kripFeed)}, nil
	}

	if kripRecipe.IsValid() || kripRecipe.Name != "" {
		return &domain.ScrapeResult{Type: domain.PageTypeRecipe, Recipe: s.mapper.toRecipe(kripRecipe)}, nil
	}

	return nil, sentinels.BadRequest("auto: could not determine content type (no valid recipe or feed found)")
}

func (s *scraperService) ScrapeRecipe(ctx context.Context, url string) (*domain.Recipe, error) {
	scrapeCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	kripRecipe, err := s.provider.ScrapeUrl(url, krip.ScrapeOptions{
		RequestOptions: defaultRequestOptions(scrapeCtx),
	})
	if err != nil {
		return nil, scrapeErr("scrape recipe", err)
	}
	return s.mapper.toRecipe(kripRecipe), nil
}

func (s *scraperService) ScrapeFeed(ctx context.Context, feed *domain.Feed, opts krip.FeedOptions) ([]*domain.Recipe, error) {
	scrapeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	opts.RequestOptions = defaultRequestOptions(scrapeCtx)
	scrapedFeed, err := s.provider.ScrapeFeedUrl(feed.Url, opts)
	if err != nil {
		return nil, scrapeErr("scrape feed", err)
	}

	recipes := make([]*domain.Recipe, 0, len(scrapedFeed.Entries))
	for _, entry := range scrapedFeed.Entries {
		recipes = append(recipes, s.mapper.toRecipe(entry))
	}

	// Back-populate the feed with scraped metadata.
	if scrapedFeed.Name != "" && feed.Name != scrapedFeed.Name {
		feed.Name = scrapedFeed.Name
	}
	if scrapedFeed.Url != "" && feed.Url != scrapedFeed.Url {
		feed.Url = utils.NormalizeURL(scrapedFeed.Url)
	}
	if scrapedFeed.Publisher != nil && feed.Publisher == nil {
		feed.Publisher = s.mapper.toPublisher(scrapedFeed.Publisher)
	}
	if scrapedFeed.Description != "" && (feed.Description == nil || *feed.Description != scrapedFeed.Description) {
		feed.Description = new(scrapedFeed.Description)
	}
	if scrapedFeed.Discovered != nil {
		feed.Discovered = scrapedFeed.Discovered
	}

	return recipes, nil
}
