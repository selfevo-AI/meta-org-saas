package tenantprojection

import (
	"testing"
	"time"
)

func TestNewWorkerAppliesProjectionDefaults(t *testing.T) {
	worker := NewWorker(nil, nil, nil, nil, WorkerConfig{WorkerID: "projection-test"})

	if worker.config.PollInterval != 2*time.Second {
		t.Fatalf("poll interval = %s, want 2s", worker.config.PollInterval)
	}
	if worker.config.LeaseDuration != time.Minute {
		t.Fatalf("lease duration = %s, want 1m", worker.config.LeaseDuration)
	}
	if worker.config.RetryDelay != 5*time.Second {
		t.Fatalf("retry delay = %s, want 5s", worker.config.RetryDelay)
	}
	if worker.config.BatchSize != 100 || worker.config.TargetLimit != 100 || worker.config.ActivityLimit != 50 || worker.config.MaxAttempts != 20 {
		t.Fatalf("worker capacity defaults = %d/%d/%d/%d", worker.config.BatchSize, worker.config.TargetLimit, worker.config.ActivityLimit, worker.config.MaxAttempts)
	}
	if stats := worker.Stats(); stats.WorkerID != "projection-test" {
		t.Fatalf("worker ID = %q, want projection-test", stats.WorkerID)
	}
}

func TestMergeMetadataDoesNotMutateInput(t *testing.T) {
	base := map[string]any{"existing": "value"}
	merged := mergeMetadata(base, map[string]any{"projection": "ready"})

	if merged["existing"] != "value" || merged["projection"] != "ready" {
		t.Fatalf("merged metadata = %#v", merged)
	}
	if _, exists := base["projection"]; exists {
		t.Fatalf("base metadata mutated: %#v", base)
	}
}
