package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/gofiber/fiber/v3/log"
)

// Job is the interface that scheduled tasks must implement.
type Job interface {
	JobType() string
	Run(ctx context.Context) (any, error)
}

type Scheduler struct {
	cron gocron.Scheduler
}

func New() (*Scheduler, error) {
	cron, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}
	return &Scheduler{cron: cron}, nil
}

func (s *Scheduler) Register(job Job, interval time.Duration) error {
	_, err := s.cron.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(func() {
			if _, err := job.Run(context.Background()); err != nil {
				log.Errorw("job execution failed", "job_type", job.JobType(), "error", err.Error())
			}
		}),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("failed to register job %s: %w", job.JobType(), err)
	}
	return nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Shutdown() error {
	return s.cron.Shutdown()
}
