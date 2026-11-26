package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/rapide-go/rapide/database/query"
)

// ProcessNext claims and runs the next ready job on queueName.
func (s *SQLStore) ProcessNext(ctx context.Context, queueName string) (bool, error) {
	if queueName == "" {
		queueName = "default"
	}
	job, err := s.jobs(ctx).
		Where("queue", "=", queueName).
		Where("available_at", "<=", time.Now().UTC().Unix()).
		OrderBy("id", "ASC").
		First()
	if err == query.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	s.mu.RLock()
	handler := s.handlers[job.Name]
	s.mu.RUnlock()

	if handler == nil {
		return true, s.moveToFailed(ctx, job, fmt.Errorf("queue: no handler registered for %q", job.Name))
	}

	if err := handler(ctx, []byte(job.Payload)); err != nil {
		nextAttempts := job.Attempts + 1
		if nextAttempts >= job.MaxAttempts {
			return true, s.moveToFailed(ctx, job, err)
		}
		backoff := time.Duration(nextAttempts*nextAttempts) * time.Second
		_, uerr := s.jobs(ctx).Where("id", "=", job.ID).Update(map[string]any{
			"attempts":     nextAttempts,
			"available_at": time.Now().UTC().Add(backoff).Unix(),
		})
		return true, uerr
	}

	_, err = s.jobs(ctx).Where("id", "=", job.ID).Delete()
	return true, err
}

func (s *SQLStore) moveToFailed(ctx context.Context, job StoredJob, runErr error) error {
	if _, err := s.failed(ctx).Create(FailedJob{
		Queue:    job.Queue,
		Name:     job.Name,
		Payload:  job.Payload,
		Error:    runErr.Error(),
		FailedAt: time.Now().UTC().Unix(),
	}); err != nil {
		return err
	}
	_, err := s.jobs(ctx).Where("id", "=", job.ID).Delete()
	return err
}

// FailedJobs lists all records currently in `failed_jobs`.
func (s *SQLStore) FailedJobs(ctx context.Context) ([]FailedJob, error) {
	return s.failed(ctx).OrderBy("id", "DESC").Get()
}

// RetryFailed moves a failed job by ID back onto the active queue.
func (s *SQLStore) RetryFailed(ctx context.Context, failedID int64) error {
	fj, err := s.failed(ctx).Find(failedID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Unix()
	if _, err := s.jobs(ctx).Create(StoredJob{
		Queue:       fj.Queue,
		Name:        fj.Name,
		Payload:     fj.Payload,
		MaxAttempts: 3,
		AvailableAt: now,
		CreatedAt:   now,
	}); err != nil {
		return err
	}
	_, err = s.failed(ctx).Where("id", "=", failedID).Delete()
	return err
}

// RunWorker polls queueName until ctx is canceled.
func (s *SQLStore) RunWorker(ctx context.Context, queueName string, pollInterval time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = 250 * time.Millisecond
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		processed, err := s.ProcessNext(ctx, queueName)
		if err != nil || !processed {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(pollInterval):
			}
		}
	}
}
