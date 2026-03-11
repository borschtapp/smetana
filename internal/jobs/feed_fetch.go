package jobs

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3/log"
	"golang.org/x/sync/errgroup"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/utils"
)

type FeedFetchJob struct {
	service          domain.FeedService
	repo             domain.FeedRepository
	schedulerRepo    domain.SchedulerRepository
	fetchConcurrency int
}

func NewFeedFetchJob(service domain.FeedService, repo domain.FeedRepository, schedulerRepo domain.SchedulerRepository) *FeedFetchJob {
	return &FeedFetchJob{
		service:          service,
		repo:             repo,
		schedulerRepo:    schedulerRepo,
		fetchConcurrency: utils.GetenvInt("FETCH_CONCURRENCY", 5),
	}
}

func (j *FeedFetchJob) JobType() string {
	return "feed_fetch"
}

func (j *FeedFetchJob) Run(ctx context.Context) (any, error) {
	feeds, err := j.repo.ListActive()
	if err != nil {
		return nil, err
	}

	log.Infow("checking feeds for updates", "count", len(feeds))

	var g errgroup.Group
	g.SetLimit(j.fetchConcurrency)

	for i := range feeds {
		feed := &feeds[i]
		g.Go(func() error { return j.fetchOne(ctx, feed) })
	}

	if err := g.Wait(); err != nil {
		log.Warnw("feed fetch completed with errors", "error", err)
		return nil, err
	}
	return nil, nil
}

func (j *FeedFetchJob) fetchOne(ctx context.Context, feed *domain.Feed) error {
	logRecord := &domain.SchedulerLog{
		JobType:   j.JobType(),
		EntityID:  &feed.ID,
		StartedAt: time.Now(),
		Status:    domain.JobStatusRunning,
	}
	if err := j.schedulerRepo.CreateLog(logRecord); err != nil {
		log.Warnw("failed to create scheduler log", "feed", feed.Url, "error", err)
	}

	found, imported, fetchErr := j.service.FetchFeed(ctx, feed)

	logRecord.CompletedAt = new(time.Now())
	if fetchErr != nil {
		logRecord.Status = domain.JobStatusError
		logRecord.ErrorMessage = fetchErr.Error()
		log.Warnw("feed fetch failed", "feed", feed, "error", fetchErr)
	} else {
		logRecord.Status = domain.JobStatusSuccess
		log.Infow("feed fetched", "url", feed.Url, "found", found, "imported", imported)
	}

	if err := j.schedulerRepo.UpdateLog(logRecord); err != nil {
		log.Warnw("failed to update scheduler log", "feed", feed.Url, "error", err)
	}
	return fetchErr
}
