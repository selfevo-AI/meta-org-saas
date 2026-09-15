package documentimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type Repository interface {
	Transact(context.Context, func(Repository, erp.Repository) error) error
	Create(context.Context, *Import, []SourceFile) (*Import, error)
	Get(context.Context, uuid.UUID) (*Import, error)
	List(context.Context, string, string, string, int) (*Page, error)
	Files(context.Context, uuid.UUID) ([]SourceFile, error)
	Save(context.Context, *Import, string, uuid.UUID) error
	Events(context.Context, uuid.UUID) ([]Event, error)
}

type PostgresRepository struct {
	db   tenantdb.DB
	inTx bool
}

func NewRepository(db tenantdb.DB) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) Transact(ctx context.Context, fn func(Repository, erp.Repository) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	// Confirmation joins the same tenant-local serialization boundary as ERP actions.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(current_database() || ':erp:ledger', 0))`); err != nil {
		return err
	}
	if err := fn(&PostgresRepository{db: tx, inTx: true}, erp.NewRepositoryWithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Create(ctx context.Context, item *Import, files []SourceFile) (*Import, error) {
	var result *Import
	err := r.Transact(ctx, func(store Repository, _ erp.Repository) error {
		tx := store.(*PostgresRepository)
		draft, err := json.Marshal(item.Draft)
		if err != nil {
			return err
		}
		var id uuid.UUID
		err = tx.db.QueryRow(ctx, `INSERT INTO ontology_document_imports (id,object_type,source_hash,draft,created_by)
			VALUES ($1,$2,$3,$4,$5) ON CONFLICT (object_type,source_hash) DO NOTHING RETURNING id`,
			item.ID, item.ObjectType, item.SourceHash, draft, item.CreatedBy).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.db.QueryRow(ctx, `SELECT id FROM ontology_document_imports WHERE object_type=$1 AND source_hash=$2`, item.ObjectType, item.SourceHash).Scan(&id); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			for ordinal, file := range files {
				if _, err := tx.db.Exec(ctx, `INSERT INTO ontology_source_files (id,import_id,name,media_type,byte_size,sha256,content,ordinal)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, file.ID, id, file.Name, file.MediaType, file.Size, file.SHA256, file.Content, ordinal); err != nil {
					return err
				}
			}
			if err := tx.appendEvent(ctx, item, "uploaded", item.CreatedBy); err != nil {
				return err
			}
		}
		result, err = tx.Get(ctx, id)
		return err
	})
	return result, err
}

const importColumns = `i.id,i.object_type,i.source_hash,i.status,i.version,i.draft,i.extraction,
	i.recognition_method,i.recognition_token,i.recognition_started_at,i.invocation_id,i.error_code,
	i.created_by,i.reviewed_by,i.reviewed_at,COALESCE(i.confirmed_key,''),i.confirmed_version,
	COALESCE(i.confirmation_hash,''),i.created_at,i.updated_at,
	COALESCE((SELECT jsonb_agg(jsonb_build_object('id',f.id,'import_id',f.import_id,'name',f.name,
	'media_type',f.media_type,'byte_size',f.byte_size,'sha256',f.sha256) ORDER BY f.ordinal)
	FROM ontology_source_files f WHERE f.import_id=i.id),'[]'::jsonb)`

func scanImport(row interface{ Scan(...any) error }) (*Import, error) {
	var item Import
	var draft, extraction, files []byte
	err := row.Scan(&item.ID, &item.ObjectType, &item.SourceHash, &item.Status, &item.Version, &draft, &extraction,
		&item.RecognitionMethod, &item.RecognitionToken, &item.RecognitionStartedAt, &item.InvocationID, &item.ErrorCode,
		&item.CreatedBy, &item.ReviewedBy, &item.ReviewedAt, &item.ConfirmedKey, &item.ConfirmedVersion, &item.ConfirmationHash,
		&item.CreatedAt, &item.UpdatedAt, &files)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, erp.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(draft, &item.Draft); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(extraction, &item.Extraction); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(files, &item.Files); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Import, error) {
	query := `SELECT ` + importColumns + ` FROM ontology_document_imports i WHERE i.id=$1`
	if r.inTx {
		query += ` FOR UPDATE OF i`
	}
	return scanImport(r.db.QueryRow(ctx, query, id))
}

func (r *PostgresRepository) List(ctx context.Context, objectType, status, cursor string, limit int) (*Page, error) {
	var after *uuid.UUID
	if cursor != "" {
		id, err := uuid.Parse(cursor)
		if err != nil {
			return nil, issue("cursor", "invalid")
		}
		after = &id
	}
	rows, err := r.db.Query(ctx, `SELECT `+importColumns+` FROM ontology_document_imports i
		WHERE i.object_type=$1 AND ($2='' OR i.status=$2)
		AND ($3::uuid IS NULL OR (i.created_at,i.id) < (SELECT created_at,id FROM ontology_document_imports WHERE id=$3 AND object_type=$1))
		ORDER BY i.created_at DESC,i.id DESC LIMIT $4`, objectType, status, after, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &Page{Imports: []Import{}}
	for rows.Next() {
		item, err := scanImport(rows)
		if err != nil {
			return nil, err
		}
		if len(page.Imports) == limit {
			page.NextCursor = page.Imports[limit-1].ID.String()
			break
		}
		page.Imports = append(page.Imports, *item)
	}
	return page, rows.Err()
}

func (r *PostgresRepository) Files(ctx context.Context, id uuid.UUID) ([]SourceFile, error) {
	rows, err := r.db.Query(ctx, `SELECT id,import_id,name,media_type,byte_size,sha256,content FROM ontology_source_files WHERE import_id=$1 ORDER BY ordinal`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := []SourceFile{}
	for rows.Next() {
		var file SourceFile
		if err := rows.Scan(&file.ID, &file.ImportID, &file.Name, &file.MediaType, &file.Size, &file.SHA256, &file.Content); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (r *PostgresRepository) Save(ctx context.Context, item *Import, event string, actor uuid.UUID) error {
	if !r.inTx {
		return fmt.Errorf("import mutation requires a transaction")
	}
	draft, err := json.Marshal(item.Draft)
	if err != nil {
		return err
	}
	extraction, err := json.Marshal(item.Extraction)
	if err != nil {
		return err
	}
	err = r.db.QueryRow(ctx, `UPDATE ontology_document_imports SET status=$2,version=version+1,draft=$3,extraction=$4,
		recognition_method=$5,recognition_token=$6,recognition_started_at=$7,invocation_id=$8,error_code=$9,
		reviewed_by=$10,reviewed_at=$11,confirmed_key=NULLIF($12,''),confirmed_version=$13,confirmation_hash=NULLIF($14,''),updated_at=NOW()
		WHERE id=$1 AND version=$15 RETURNING version,updated_at`, item.ID, item.Status, draft, extraction, item.RecognitionMethod,
		item.RecognitionToken, item.RecognitionStartedAt, item.InvocationID, item.ErrorCode, item.ReviewedBy, item.ReviewedAt,
		item.ConfirmedKey, item.ConfirmedVersion, item.ConfirmationHash, item.Version).Scan(&item.Version, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return erp.ErrConflict
	}
	if err != nil {
		return err
	}
	return r.appendEvent(ctx, item, event, actor)
}

func (r *PostgresRepository) appendEvent(ctx context.Context, item *Import, event string, actor uuid.UUID) error {
	snapshot, err := json.Marshal(map[string]any{"draft": item.Draft, "extraction": item.Extraction,
		"confirmed_key": item.ConfirmedKey, "error_code": item.ErrorCode, "invocation_id": item.InvocationID, "recognition_method": item.RecognitionMethod})
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO ontology_import_events(import_id,version,event,actor_id,snapshot) VALUES($1,$2,$3,$4,$5)`, item.ID, item.Version, event, actor, snapshot)
	return err
}

func (r *PostgresRepository) Events(ctx context.Context, id uuid.UUID) ([]Event, error) {
	rows, err := r.db.Query(ctx, `SELECT id,version,event,actor_id,snapshot,created_at FROM ontology_import_events WHERE import_id=$1 ORDER BY id DESC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		var event Event
		var snapshot []byte
		if err := rows.Scan(&event.ID, &event.Version, &event.Event, &event.ActorID, &snapshot, &event.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(snapshot, &event.Snapshot); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
