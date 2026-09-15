package documentimport

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/ontology"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

type Recognition struct {
	Extraction   Extraction
	Method       string
	InvocationID *uuid.UUID
}

type Recognizer interface {
	Recognize(context.Context, ontology.ObjectType, []SourceFile) (*Recognition, error)
}

type Service struct {
	repo       Repository
	ontology   *ontology.Service
	business   *erp.Service
	recognizer Recognizer
}

func NewService(repo Repository, objects *ontology.Service, business *erp.Service, recognizer Recognizer) *Service {
	return &Service{repo: repo, ontology: objects, business: business, recognizer: recognizer}
}

func humanActor(ctx context.Context) (uuid.UUID, error) {
	user, ok := middleware.UserFromContext(ctx)
	if !ok || user.Type != "human" {
		return uuid.Nil, fmt.Errorf("%w: human confirmation is required", erp.ErrForbidden)
	}
	id, err := uuid.Parse(user.ID)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, erp.ErrForbidden
	}
	return id, nil
}

func (s *Service) objectType(ctx context.Context, key, operation string) (ontology.ObjectType, error) {
	typ, err := s.ontology.Type(ctx, key)
	if err != nil {
		return typ, err
	}
	if !typ.Importable {
		return typ, issue("object_type", "unsupported_type")
	}
	return typ, s.business.CheckAccess(ctx, typ.TableCode, operation)
}

func (s *Service) authorizeImport(ctx context.Context, id uuid.UUID, operation string) (ontology.ObjectType, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return ontology.ObjectType{}, err
	}
	return s.objectType(ctx, item.ObjectType, operation)
}

func (s *Service) Upload(ctx context.Context, objectType string, files []SourceFile) (*Import, error) {
	actor, err := humanActor(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.objectType(ctx, objectType, "create"); err != nil {
		return nil, err
	}
	files, hash, err := validateFiles(files)
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	item := &Import{ID: id, ObjectType: objectType, SourceHash: hash, Status: StatusUploaded, Version: 1, CreatedBy: actor,
		Draft: Draft{Key: "IMP-" + id.String(), Properties: map[string]any{}, Lines: []map[string]any{}}}
	return s.repo.Create(ctx, item, files)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Import, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.objectType(ctx, item.ObjectType, "read"); err != nil {
		return nil, err
	}
	item.Events, err = s.repo.Events(ctx, id)
	return item, err
}

func (s *Service) List(ctx context.Context, objectType, status, cursor string) (*Page, error) {
	if _, err := s.objectType(ctx, objectType, "read"); err != nil {
		return nil, err
	}
	switch status {
	case "", StatusUploaded, StatusRecognizing, StatusNeedsReview, StatusConfirmed, StatusRejected, StatusFailed:
	default:
		return nil, issue("status", "invalid")
	}
	return s.repo.List(ctx, objectType, status, cursor, 50)
}

func (s *Service) File(ctx context.Context, id, fileID uuid.UUID) (*SourceFile, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.objectType(ctx, item.ObjectType, "read"); err != nil {
		return nil, err
	}
	files, err := s.repo.Files(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.ID == fileID {
			return &file, nil
		}
	}
	return nil, erp.ErrNotFound
}

func (s *Service) Recognize(ctx context.Context, id uuid.UUID, version int) (*Import, error) {
	actor, err := humanActor(ctx)
	if err != nil {
		return nil, err
	}
	typ, err := s.authorizeImport(ctx, id, "create")
	if err != nil {
		return nil, err
	}
	var started *Import
	token := uuid.New()
	err = s.repo.Transact(ctx, func(repo Repository, _ erp.Repository) error {
		item, err := repo.Get(ctx, id)
		if err != nil {
			return err
		}
		if item.Version != version || item.Status == StatusConfirmed || item.Status == StatusRejected {
			return erp.ErrConflict
		}
		if item.Status == StatusRecognizing && item.RecognitionStartedAt != nil && time.Since(*item.RecognitionStartedAt) < 3*time.Minute {
			return erp.ErrConflict
		}
		now := time.Now().UTC()
		item.Status, item.RecognitionToken, item.RecognitionStartedAt, item.ErrorCode = StatusRecognizing, &token, &now, ""
		if err := repo.Save(ctx, item, "recognizing", actor); err != nil {
			return err
		}
		started = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	recognitionCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	files, recognitionErr := s.repo.Files(recognitionCtx, id)
	var recognition *Recognition
	if recognitionErr == nil {
		if s.recognizer == nil {
			recognitionErr = ErrRecognitionUnavailable
		} else {
			recognition, recognitionErr = s.recognizer.Recognize(recognitionCtx, typ, files)
		}
	}
	if recognitionErr == nil && recognition == nil {
		recognitionErr = fmt.Errorf("empty recognition response")
	}
	var draft Draft
	if recognitionErr == nil {
		recognition.Extraction.Draft.Key = started.Draft.Key
		draft, recognitionErr = normalizeDraft(recognition.Extraction.Draft, typ.Key, false)
	}
	// A disconnected client must not leave an otherwise finished recognition leased.
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer finishCancel()
	var result *Import
	err = s.repo.Transact(finishCtx, func(repo Repository, _ erp.Repository) error {
		item, err := repo.Get(finishCtx, id)
		if err != nil {
			return err
		}
		if item.Status != StatusRecognizing || item.RecognitionToken == nil || *item.RecognitionToken != token {
			return erp.ErrConflict
		}
		item.RecognitionToken = nil
		event := "recognized"
		if recognition != nil {
			item.Extraction, item.RecognitionMethod, item.InvocationID = recognition.Extraction, recognition.Method, recognition.InvocationID
		}
		if recognitionErr != nil {
			item.Status, item.ErrorCode, event = StatusFailed, "recognition_failed", "recognition_failed"
			if errors.Is(recognitionErr, ErrRecognitionUnavailable) {
				item.ErrorCode = "recognition_unavailable"
			}
		} else {
			item.Status, item.ErrorCode = StatusNeedsReview, ""
			if item.ReviewedBy == nil {
				item.Draft = draft
			}
		}
		if err := repo.Save(finishCtx, item, event, actor); err != nil {
			return err
		}
		result = item
		return nil
	})
	return result, err
}

func (s *Service) Review(ctx context.Context, id uuid.UUID, input ReviewInput) (*Import, error) {
	actor, err := humanActor(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.authorizeImport(ctx, id, "create"); err != nil {
		return nil, err
	}
	var result *Import
	err = s.repo.Transact(ctx, func(repo Repository, _ erp.Repository) error {
		item, err := repo.Get(ctx, id)
		if err != nil {
			return err
		}
		if item.Version != input.Version || !editableImport(item.Status) {
			return erp.ErrConflict
		}
		draft, err := normalizeDraft(input.Draft, item.ObjectType, false)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		item.Draft, item.Status, item.ReviewedBy, item.ReviewedAt = draft, StatusNeedsReview, &actor, &now
		if err := repo.Save(ctx, item, "reviewed", actor); err != nil {
			return err
		}
		result = item
		return nil
	})
	return result, err
}

func (s *Service) Reject(ctx context.Context, id uuid.UUID, version int) (*Import, error) {
	actor, err := humanActor(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.authorizeImport(ctx, id, "create"); err != nil {
		return nil, err
	}
	var result *Import
	err = s.repo.Transact(ctx, func(repo Repository, _ erp.Repository) error {
		item, err := repo.Get(ctx, id)
		if err != nil {
			return err
		}
		if item.Version != version || !editableImport(item.Status) {
			return erp.ErrConflict
		}
		now := time.Now().UTC()
		item.Status, item.ReviewedBy, item.ReviewedAt = StatusRejected, &actor, &now
		if err := repo.Save(ctx, item, "rejected", actor); err != nil {
			return err
		}
		result = item
		return nil
	})
	return result, err
}

func (s *Service) Confirm(ctx context.Context, id uuid.UUID, input ReviewInput) (*Import, error) {
	actor, err := humanActor(ctx)
	if err != nil {
		return nil, err
	}
	if !input.Confirmed {
		return nil, issue("confirmed", "confirmation_required")
	}
	typ, err := s.authorizeImport(ctx, id, "create")
	if err != nil {
		return nil, err
	}
	var result *Import
	err = s.repo.Transact(ctx, func(repo Repository, businessRepo erp.Repository) error {
		item, err := repo.Get(ctx, id)
		if err != nil {
			return err
		}
		draft, err := normalizeDraft(input.Draft, item.ObjectType, true)
		if err != nil {
			return err
		}
		encoded, _ := json.Marshal(draft)
		hash := fmt.Sprintf("%x", sha256.Sum256(encoded))
		if item.Status == StatusConfirmed {
			if item.ConfirmationHash != hash || item.ConfirmedVersion == nil || *item.ConfirmedVersion != input.Version {
				return erp.ErrConflict
			}
			result = item
			return nil
		}
		if item.Version != input.Version || !editableImport(item.Status) {
			return erp.ErrConflict
		}
		business := erp.NewService(businessRepo, s.business.Catalog(ctx))
		if err := validateReferences(ctx, business, draft, typ.Key); err != nil {
			return err
		}
		record, err := business.CreateRecord(ctx, typ.TableCode, erp.RecordInput{Key: draft.Key, Data: mapHeader(draft, typ)})
		if err != nil {
			return err
		}
		table, _ := s.business.Catalog(ctx).Table(typ.TableCode)
		for index, line := range draft.Lines {
			if len(table.Children) == 0 {
				return issue("lines", "unsupported_type")
			}
			data := map[string]any{}
			for key, value := range line {
				if key == "quantity" || key == "unit_price" || key == "tax_rate" {
					value = json.Number(value.(string))
				}
				data[lineFields[key]] = value
			}
			if _, err := business.CreateChildRecord(ctx, typ.TableCode, record.Key, table.Children[0].Code, erp.RecordInput{Key: strconv.Itoa(index + 1), Data: data}); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		provenance := map[string]any{"source": "external_document", "import_id": item.ID.String(), "source_hash": item.SourceHash,
			"reviewed_by": actor.String(), "reviewed_at": now.Format(time.RFC3339Nano), "recognition_method": item.RecognitionMethod}
		if _, err := businessRepo.UpdateRecord(ctx, table, record.Key, erp.RecordInput{Data: map[string]any{"provenance": provenance}}); err != nil {
			return err
		}
		execution, err := businessRepo.CreateActionExecution(ctx, erp.ActionExecution{ID: uuid.New(), TableCode: typ.TableCode, RecordKey: record.Key,
			Action: "import", Status: "running", IdempotencyKey: "document-import:" + item.ID.String(), ActorID: &actor, ActorType: "human", Source: "document_import", Payload: provenance})
		if err != nil {
			return err
		}
		if _, err := businessRepo.CompleteActionExecution(ctx, execution.ID, "completed", provenance, nil); err != nil {
			return err
		}
		item.Draft, item.Status, item.ReviewedBy, item.ReviewedAt = draft, StatusConfirmed, &actor, &now
		item.ConfirmedKey, item.ConfirmedVersion, item.ConfirmationHash = record.Key, &input.Version, hash
		if err := repo.Save(ctx, item, "confirmed", actor); err != nil {
			return err
		}
		result = item
		return nil
	})
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return nil, fmt.Errorf("%w: document key already exists", erp.ErrConflict)
	}
	return result, err
}

func editableImport(status string) bool {
	return status == StatusUploaded || status == StatusNeedsReview || status == StatusFailed
}
