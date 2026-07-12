package auditretention

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

type retentionRepository interface {
	RedactBatch(context.Context, time.Time, int) (RedactionCounts, error)
	RecordRun(context.Context, RunRecord) error
}

type WorkerConfig struct {
	Enabled       bool
	RetentionDays int
	PollInterval  time.Duration
	BatchSize     int
	MaxBatches    int
}

type WorkerStats struct {
	Running           bool            `json:"running"`
	LastStartedAt     time.Time       `json:"last_started_at"`
	LastCompletedAt   time.Time       `json:"last_completed_at"`
	LastCutoffAt      time.Time       `json:"last_cutoff_at"`
	LastCounts        RedactionCounts `json:"last_counts"`
	RunsTotal         uint64          `json:"runs_total"`
	FailuresTotal     uint64          `json:"failures_total"`
	RowsRedactedTotal uint64          `json:"rows_redacted_total"`
	LastError         string          `json:"last_error,omitempty"`
}

type Worker struct {
	repo   retentionRepository
	config WorkerConfig
	now    func() time.Time
	mu     sync.Mutex
	stats  WorkerStats
}

func NewWorker(repo retentionRepository, config WorkerConfig) *Worker {
	if config.RetentionDays <= 0 {
		config.RetentionDays = 365
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 24 * time.Hour
	}
	if config.BatchSize <= 0 || config.BatchSize > 5000 {
		config.BatchSize = 500
	}
	if config.MaxBatches <= 0 || config.MaxBatches > 1000 {
		config.MaxBatches = 20
	}
	return &Worker{repo: repo, config: config, now: time.Now}
}

func (w *Worker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || !w.config.Enabled {
		return
	}
	w.setRunning(true)
	go func() {
		defer w.setRunning(false)
		if _, err := w.RunOnce(ctx); err != nil {
			log.Printf("audit retention run failed: %v", err)
		}
		ticker := time.NewTicker(w.config.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := w.RunOnce(ctx); err != nil {
					log.Printf("audit retention run failed: %v", err)
				}
			}
		}
	}()
}

func (w *Worker) RunOnce(ctx context.Context) (RedactionCounts, error) {
	started := w.now().UTC()
	cutoff := started.AddDate(0, 0, -w.config.RetentionDays)
	w.beginRun(started, cutoff)
	var total RedactionCounts
	var runErr error
	for batch := 0; batch < w.config.MaxBatches; batch++ {
		counts, err := w.repo.RedactBatch(ctx, cutoff, w.config.BatchSize)
		if err != nil {
			runErr = err
			break
		}
		total.Add(counts)
		if counts.Total() == 0 {
			break
		}
	}
	status := "succeeded"
	errorMessage := ""
	if runErr != nil {
		status = "failed"
		errorMessage = runErr.Error()
	}
	recordErr := w.repo.RecordRun(ctx, RunRecord{
		StartedAt: started, CutoffAt: cutoff, RetentionDays: w.config.RetentionDays,
		BatchSize: w.config.BatchSize, Status: status, Counts: total, ErrorMessage: errorMessage,
	})
	if recordErr != nil {
		runErr = errors.Join(runErr, recordErr)
	}
	w.finishRun(total, runErr)
	return total, runErr
}

func (w *Worker) Stats() WorkerStats {
	if w == nil {
		return WorkerStats{}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stats
}

func (w *Worker) setRunning(running bool) {
	w.mu.Lock()
	w.stats.Running = running
	w.mu.Unlock()
}

func (w *Worker) beginRun(started time.Time, cutoff time.Time) {
	w.mu.Lock()
	w.stats.LastStartedAt = started
	w.stats.LastCutoffAt = cutoff
	w.mu.Unlock()
}

func (w *Worker) finishRun(counts RedactionCounts, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stats.LastCompletedAt = w.now().UTC()
	w.stats.LastCounts = counts
	w.stats.RunsTotal++
	w.stats.RowsRedactedTotal += uint64(counts.Total())
	w.stats.LastError = ""
	if err != nil {
		w.stats.FailuresTotal++
		w.stats.LastError = err.Error()
	}
}
