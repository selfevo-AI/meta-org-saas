package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateTrace(ctx context.Context, input CreateTraceInput) (*Trace, error) {
	metadata, _ := json.Marshal(input.Metadata)
	t := &Trace{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO traces (workflow_id, status, metadata)
		 VALUES ($1, $2, $3)
		 RETURNING id, workflow_id, status, started_at, completed_at, metadata`,
		input.WorkflowID, TraceActive, metadata,
	).Scan(&t.ID, &t.WorkflowID, &t.Status, &t.StartedAt, &t.CompletedAt, &metadata)
	if err != nil {
		return nil, fmt.Errorf("create trace: %w", err)
	}
	json.Unmarshal(metadata, &t.Metadata)
	return t, nil
}

func (r *Repository) GetTrace(ctx context.Context, id uuid.UUID) (*Trace, error) {
	t := &Trace{}
	var metadata []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, workflow_id, status, started_at, completed_at, metadata
		 FROM traces WHERE id = $1`, id,
	).Scan(&t.ID, &t.WorkflowID, &t.Status, &t.StartedAt, &t.CompletedAt, &metadata)
	if err != nil {
		return nil, fmt.Errorf("get trace: %w", err)
	}
	json.Unmarshal(metadata, &t.Metadata)
	return t, nil
}

func (r *Repository) ListTraces(ctx context.Context, limit int) ([]Trace, error) {
	limit = normalizeLimit(limit, 50, 100)
	rows, err := r.db.Query(ctx,
		`SELECT id, workflow_id, status, started_at, completed_at, metadata
		 FROM traces ORDER BY started_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list traces: %w", err)
	}
	defer rows.Close()

	var traces []Trace
	for rows.Next() {
		var t Trace
		var metadata []byte
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Status, &t.StartedAt, &t.CompletedAt, &metadata); err != nil {
			return nil, fmt.Errorf("scan trace: %w", err)
		}
		json.Unmarshal(metadata, &t.Metadata)
		traces = append(traces, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list traces iteration: %w", err)
	}
	return traces, nil
}

func (r *Repository) RecordSpan(ctx context.Context, input RecordSpanInput) (*Span, error) {
	inputJSON, _ := json.Marshal(input.Input)
	outputJSON, _ := json.Marshal(input.Output)
	metadata, _ := json.Marshal(input.Metadata)

	s := &Span{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO spans (trace_id, parent_span_id, span_type, entity_id, entity_type, actor_id, actor_type, input, output, started_at, completed_at, duration_ms, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, trace_id, parent_span_id, span_type, entity_id, entity_type, actor_id, actor_type, input, output, started_at, completed_at, duration_ms, metadata`,
		input.TraceID, input.ParentSpanID, input.SpanType, input.EntityID, input.EntityType, input.ActorID, input.ActorType, inputJSON, outputJSON, time.Now(), nil, input.DurationMs, metadata,
	).Scan(&s.ID, &s.TraceID, &s.ParentSpanID, &s.SpanType, &s.EntityID, &s.EntityType, &s.ActorID, &s.ActorType, &inputJSON, &outputJSON, &s.StartedAt, &s.CompletedAt, &s.DurationMs, &metadata)
	if err != nil {
		return nil, fmt.Errorf("record span: %w", err)
	}
	json.Unmarshal(inputJSON, &s.Input)
	json.Unmarshal(outputJSON, &s.Output)
	json.Unmarshal(metadata, &s.Metadata)
	return s, nil
}

func (r *Repository) GetSpansByTrace(ctx context.Context, traceID uuid.UUID) ([]Span, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, trace_id, parent_span_id, span_type, entity_id, entity_type, actor_id, actor_type, input, output, started_at, completed_at, duration_ms, metadata
		 FROM spans WHERE trace_id = $1 ORDER BY started_at`, traceID)
	if err != nil {
		return nil, fmt.Errorf("get spans by trace: %w", err)
	}
	defer rows.Close()

	var spans []Span
	for rows.Next() {
		var s Span
		var inputJSON, outputJSON, metadata []byte
		if err := rows.Scan(&s.ID, &s.TraceID, &s.ParentSpanID, &s.SpanType, &s.EntityID, &s.EntityType, &s.ActorID, &s.ActorType, &inputJSON, &outputJSON, &s.StartedAt, &s.CompletedAt, &s.DurationMs, &metadata); err != nil {
			return nil, fmt.Errorf("scan span: %w", err)
		}
		json.Unmarshal(inputJSON, &s.Input)
		json.Unmarshal(outputJSON, &s.Output)
		json.Unmarshal(metadata, &s.Metadata)
		spans = append(spans, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get spans by trace iteration: %w", err)
	}
	return spans, nil
}

func (r *Repository) RecordMetric(ctx context.Context, input RecordMetricInput) (*Metric, error) {
	metadata, _ := json.Marshal(input.Metadata)
	m := &Metric{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO metrics (metric_type, metric_name, entity_id, entity_type, value, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, metric_type, metric_name, entity_id, entity_type, value, recorded_at, metadata`,
		input.MetricType, input.MetricName, input.EntityID, input.EntityType, input.Value, metadata,
	).Scan(&m.ID, &m.MetricType, &m.MetricName, &m.EntityID, &m.EntityType, &m.Value, &m.RecordedAt, &metadata)
	if err != nil {
		return nil, fmt.Errorf("record metric: %w", err)
	}
	json.Unmarshal(metadata, &m.Metadata)
	return m, nil
}

// normalizeLimit clamps a caller-supplied limit to [defaultLimit..maxLimit],
// falling back to the default for non-positive values.
func normalizeLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// buildMetricsQuery assembles the filtered metrics statement and its bound
// arguments; it is pure so the filter/limit rules can be tested without a
// database.
func buildMetricsQuery(q MetricsQuery, limit int) (string, []any) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if q.MetricType != "" {
		where += fmt.Sprintf(" AND metric_type = $%d", argIdx)
		args = append(args, q.MetricType)
		argIdx++
	}
	if q.MetricName != "" {
		where += fmt.Sprintf(" AND metric_name = $%d", argIdx)
		args = append(args, q.MetricName)
		argIdx++
	}
	if q.EntityID != nil {
		where += fmt.Sprintf(" AND entity_id = $%d", argIdx)
		args = append(args, *q.EntityID)
		argIdx++
	}
	if q.From != nil {
		where += fmt.Sprintf(" AND recorded_at >= $%d", argIdx)
		args = append(args, *q.From)
		argIdx++
	}
	if q.To != nil {
		where += fmt.Sprintf(" AND recorded_at <= $%d", argIdx)
		args = append(args, *q.To)
		argIdx++
	}

	query := fmt.Sprintf(`SELECT id, metric_type, metric_name, entity_id, entity_type, value, recorded_at, metadata
		FROM metrics %s ORDER BY recorded_at DESC LIMIT $%d`, where, argIdx)
	args = append(args, limit)
	return query, args
}

func (r *Repository) QueryMetrics(ctx context.Context, q MetricsQuery) ([]Metric, error) {
	limit := normalizeLimit(q.Limit, 50, 500)
	query, args := buildMetricsQuery(q, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()

	var metrics []Metric
	for rows.Next() {
		var m Metric
		var metadata []byte
		if err := rows.Scan(&m.ID, &m.MetricType, &m.MetricName, &m.EntityID, &m.EntityType, &m.Value, &m.RecordedAt, &metadata); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}
		json.Unmarshal(metadata, &m.Metadata)
		metrics = append(metrics, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query metrics iteration: %w", err)
	}
	return metrics, nil
}

func (r *Repository) CompleteTrace(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE traces SET status = $1, completed_at = NOW() WHERE id = $2`,
		status, id)
	if err != nil {
		return fmt.Errorf("complete trace: %w", err)
	}
	return nil
}
