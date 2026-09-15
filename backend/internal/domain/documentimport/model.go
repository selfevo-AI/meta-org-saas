package documentimport

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
)

const (
	MaxFiles          = 5
	MaxFileBytes      = 10 * 1024 * 1024
	MaxTotalBytes     = 20 * 1024 * 1024
	MaxLines          = 200
	MaxTextBytes      = 120000
	StatusUploaded    = "uploaded"
	StatusRecognizing = "recognizing"
	StatusNeedsReview = "needs_review"
	StatusConfirmed   = "confirmed"
	StatusRejected    = "rejected"
	StatusFailed      = "failed"
)

var ErrRecognitionUnavailable = errors.New("document recognition is unavailable")

type Draft struct {
	Key        string           `json:"key"`
	Properties map[string]any   `json:"properties"`
	Lines      []map[string]any `json:"lines"`
}

type Evidence struct {
	Path       string    `json:"path"`
	SourceID   uuid.UUID `json:"source_id"`
	Page       int       `json:"page,omitempty"`
	Quote      string    `json:"quote"`
	Confidence float64   `json:"confidence"`
}

type Extraction struct {
	Draft    Draft      `json:"draft"`
	Evidence []Evidence `json:"evidence"`
	Warnings []string   `json:"warnings"`
}

type SourceFile struct {
	ID        uuid.UUID `json:"id"`
	ImportID  uuid.UUID `json:"import_id"`
	Name      string    `json:"name"`
	MediaType string    `json:"media_type"`
	Size      int       `json:"byte_size"`
	SHA256    string    `json:"sha256"`
	Content   []byte    `json:"-"`
}

type Event struct {
	ID        int64          `json:"id"`
	Version   int            `json:"version"`
	Event     string         `json:"event"`
	ActorID   uuid.UUID      `json:"actor_id"`
	Snapshot  map[string]any `json:"snapshot"`
	CreatedAt time.Time      `json:"created_at"`
}

type Import struct {
	ID                   uuid.UUID    `json:"id"`
	ObjectType           string       `json:"object_type"`
	SourceHash           string       `json:"source_hash"`
	Status               string       `json:"status"`
	Version              int          `json:"version"`
	Draft                Draft        `json:"draft"`
	Extraction           Extraction   `json:"extraction"`
	RecognitionMethod    string       `json:"recognition_method"`
	InvocationID         *uuid.UUID   `json:"invocation_id,omitempty"`
	ErrorCode            string       `json:"error_code,omitempty"`
	CreatedBy            uuid.UUID    `json:"created_by"`
	ReviewedBy           *uuid.UUID   `json:"reviewed_by,omitempty"`
	ReviewedAt           *time.Time   `json:"reviewed_at,omitempty"`
	ConfirmedKey         string       `json:"confirmed_key,omitempty"`
	CreatedAt            time.Time    `json:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at"`
	Files                []SourceFile `json:"files"`
	Events               []Event      `json:"events,omitempty"`
	RecognitionToken     *uuid.UUID   `json:"-"`
	RecognitionStartedAt *time.Time   `json:"recognition_started_at,omitempty"`
	ConfirmedVersion     *int         `json:"-"`
	ConfirmationHash     string       `json:"-"`
}

type ReviewInput struct {
	Version   int   `json:"version"`
	Draft     Draft `json:"draft"`
	Confirmed bool  `json:"confirmed"`
}

type VersionInput struct {
	Version int `json:"version"`
}

type Page struct {
	Imports    []Import `json:"imports"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

type Issue struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

type ValidationError struct{ Issues []Issue }

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: document fields need review", erp.ErrValidation)
}
func (e *ValidationError) Unwrap() error { return erp.ErrValidation }

func issue(path, code string) error {
	return &ValidationError{Issues: []Issue{{Path: path, Code: code}}}
}
