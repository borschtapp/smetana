package jobs

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/domain"
)

// SuggestionReconcileJob runs all registered global detectors on a watermark sweep.
// Each detector examines entities updated since its last successful run.
type SuggestionReconcileJob struct {
	service       domain.SuggestionService
	schedulerRepo domain.SchedulerRepository
}

func NewSuggestionReconcileJob(service domain.SuggestionService, schedulerRepo domain.SchedulerRepository) *SuggestionReconcileJob {
	return &SuggestionReconcileJob{service: service, schedulerRepo: schedulerRepo}
}

func (j *SuggestionReconcileJob) JobType() string {
	return "suggestion_reconcile"
}

func (j *SuggestionReconcileJob) Run(ctx context.Context) (any, error) {
	logRecord := &domain.SchedulerLog{
		JobType:   j.JobType(),
		StartedAt: time.Now(),
		Status:    domain.JobStatusRunning,
	}
	if err := j.schedulerRepo.CreateLog(logRecord); err != nil {
		log.Errorw("failed to create scheduler log, skipping reconcile sweep", "error", err.Error())
		return nil, err
	}

	reconcileErr := j.service.Reconcile(ctx, nil)

	logRecord.CompletedAt = new(time.Now())
	if reconcileErr != nil {
		logRecord.Status = domain.JobStatusError
		logRecord.ErrorMessage = reconcileErr.Error()
		log.Errorw("suggestion reconcile sweep failed", "error", reconcileErr)
	} else {
		logRecord.Status = domain.JobStatusSuccess
		log.Infow("suggestion reconcile sweep complete", "elapsed_ms", time.Since(logRecord.StartedAt).Milliseconds())
	}

	if err := j.schedulerRepo.UpdateLog(logRecord); err != nil {
		log.Warnw("failed to update scheduler log", "error", err.Error())
	}
	return nil, reconcileErr
}
