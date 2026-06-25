package monitoringagent

import (
	"context"
	"testing"
	"time"
)

func TestSchedulerIsDisabledByDefault(t *testing.T) {
	repo := &memoryRepository{}
	service := NewService(repo, ServiceConfig{MaxSignalsPerRun: 100})
	scheduler := NewScheduler(service, SchedulerConfig{})

	run, executed, err := scheduler.RunIfDue(context.Background(), time.Date(2026, 6, 25, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("RunIfDue returned error: %v", err)
	}
	if executed {
		t.Fatal("executed = true, want false")
	}
	if run != nil {
		t.Fatalf("run = %#v, want nil", run)
	}
	if len(repo.runs) != 0 {
		t.Fatalf("stored runs = %d, want 0", len(repo.runs))
	}
}

func TestSchedulerRunsOncePerDayWhenEnabledAndDue(t *testing.T) {
	repo := &memoryRepository{}
	service := NewService(repo, ServiceConfig{MaxSignalsPerRun: 100})
	scheduler := NewScheduler(service, SchedulerConfig{
		Enabled:       true,
		DailyTime:     "02:00",
		LookbackHours: 24,
	})
	now := time.Date(2026, 6, 25, 2, 5, 0, 0, time.UTC)

	run, executed, err := scheduler.RunIfDue(context.Background(), now)
	if err != nil {
		t.Fatalf("RunIfDue returned error: %v", err)
	}
	if !executed {
		t.Fatal("executed = false, want true")
	}
	if run == nil || run.TriggerType != TriggerScheduled {
		t.Fatalf("run trigger = %#v, want scheduled run", run)
	}

	secondRun, secondExecuted, err := scheduler.RunIfDue(context.Background(), now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("second RunIfDue returned error: %v", err)
	}
	if secondExecuted {
		t.Fatal("second executed = true, want false")
	}
	if secondRun != nil {
		t.Fatalf("second run = %#v, want nil", secondRun)
	}
}
