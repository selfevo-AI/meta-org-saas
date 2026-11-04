package observability

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNormalizeLimitClampsToRange(t *testing.T) {
	cases := []struct {
		limit, want int
	}{
		{0, 50},
		{-5, 50},
		{1, 1},
		{73, 73},
		{100, 100},
		{101, 100},
		{5000, 100},
	}
	for _, tc := range cases {
		if got := normalizeLimit(tc.limit, 50, 100); got != tc.want {
			t.Errorf("normalizeLimit(%d, 50, 100) = %d, want %d", tc.limit, got, tc.want)
		}
	}
	if got := normalizeLimit(600, 50, 500); got != 500 {
		t.Errorf("normalizeLimit(600, 50, 500) = %d, want 500", got)
	}
}

func TestBuildMetricsQueryWithoutFilters(t *testing.T) {
	query, args := buildMetricsQuery(MetricsQuery{}, 50)
	if !strings.Contains(query, "WHERE 1=1") {
		t.Fatalf("query missing base filter: %s", query)
	}
	if !strings.Contains(query, "ORDER BY recorded_at DESC") {
		t.Fatalf("query missing ordering: %s", query)
	}
	if !strings.Contains(query, "LIMIT $1") {
		t.Fatalf("query should bind the limit as the first argument: %s", query)
	}
	if len(args) != 1 || args[0] != 50 {
		t.Fatalf("args = %v, want only the limit", args)
	}
}

func TestBuildMetricsQueryBindsFiltersInOrder(t *testing.T) {
	entityID := uuid.New()
	from := time.Now().Add(-time.Hour)
	to := time.Now()
	query, args := buildMetricsQuery(MetricsQuery{
		MetricType: MetricType("latency"),
		MetricName: "p95",
		EntityID:   &entityID,
		From:       &from,
		To:         &to,
	}, 500)

	for _, fragment := range []string{
		"metric_type = $1",
		"metric_name = $2",
		"entity_id = $3",
		"recorded_at >= $4",
		"recorded_at <= $5",
		"LIMIT $6",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query missing %q:\n%s", fragment, query)
		}
	}
	if len(args) != 6 {
		t.Fatalf("args length = %d, want 6", len(args))
	}
	if args[0] != MetricType("latency") || args[1] != "p95" || args[2] != entityID {
		t.Fatalf("args[0..2] = %v, want metric type, name, entity", args[:3])
	}
	if args[3] != from || args[4] != to {
		t.Fatalf("args[3..4] = %v, want from and to timestamps", args[3:5])
	}
	if args[5] != 500 {
		t.Fatalf("args[5] = %v, want the limit", args[5])
	}
}
