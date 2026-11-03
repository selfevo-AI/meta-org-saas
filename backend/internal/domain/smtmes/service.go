package smtmes

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/safeurl"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/secretbox"
)

var (
	ErrValidation = errors.New("validation error")
	ErrForbidden  = errors.New("forbidden")
	ErrNotFound   = errors.New("not found")
)

// httpDoer performs one JSON HTTP request; injectable for tests.
type httpDoer func(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error)

// newHTTPDoer builds the doer used for edge pull/push calls. The client
// enforces the outbound policy at dial time so an edge whose DNS answer
// changes after validation still cannot reach internal networks.
func newHTTPDoer(policy safeurl.Policy) httpDoer {
	client := policy.Client(10 * time.Second)
	return func(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
		var reader io.Reader
		if body != nil {
			reader = strings.NewReader(string(body))
		}
		request, err := http.NewRequestWithContext(ctx, method, url, reader)
		if err != nil {
			return 0, nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		for name, value := range headers {
			request.Header.Set(name, value)
		}
		response, err := client.Do(request)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err != nil {
			return response.StatusCode, nil, err
		}
		return response.StatusCode, payload, nil
	}
}

type Service struct {
	repo     *Repository
	erp      *erp.Service
	box      *secretbox.Box
	outbound safeurl.Policy
	do       httpDoer
}

func NewService(repo *Repository, erpService *erp.Service, box *secretbox.Box) *Service {
	return &Service{repo: repo, erp: erpService, box: box, do: newHTTPDoer(safeurl.Policy{})}
}

// WithOutboundPolicy sets which destinations edge instances may point at and
// rebuilds the HTTP doer to enforce it.
func (s *Service) WithOutboundPolicy(policy safeurl.Policy) *Service {
	s.outbound = policy
	s.do = newHTTPDoer(policy)
	return s
}

// WithHTTPDoer swaps the HTTP client (tests).
func (s *Service) WithHTTPDoer(doer httpDoer) *Service {
	s.do = doer
	return s
}

// ---------------------------------------------------------------------------
// Tenant authorization helpers.
// ---------------------------------------------------------------------------

func tenantFrom(ctx context.Context) (*middleware.TenantContext, error) {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant == nil || tenant.OrganizationID == nil {
		return nil, ErrForbidden
	}
	return tenant, nil
}

func requireOrgAdmin(ctx context.Context) (*middleware.TenantContext, error) {
	tenant, err := tenantFrom(ctx)
	if err != nil {
		return nil, err
	}
	if tenant.IsPlatformAdmin {
		return tenant, nil
	}
	switch tenant.AuthorityTier {
	case "organization_creator", "organization_admin":
		return tenant, nil
	}
	return nil, ErrForbidden
}

// ---------------------------------------------------------------------------
// Settings.
// ---------------------------------------------------------------------------

func (s *Service) Settings(ctx context.Context) (*SettingsView, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	stored, err := s.erp.SMTExportRecords(ctx, "MSET", map[string]any{}, 500)
	if err != nil {
		return nil, err
	}
	defaults := erp.SMTSettingDefaults()
	values := map[string]string{}
	for key, value := range defaults {
		values[key] = value
	}
	storedValues := map[string]string{}
	for _, row := range stored {
		key, _ := row["key"].(string)
		if key == "" {
			continue
		}
		value := ""
		if raw, present := row["Value"]; present && raw != nil {
			value = fmt.Sprint(raw)
		}
		storedValues[key] = value
		values[key] = value
	}
	return &SettingsView{Values: values, Stored: storedValues}, nil
}

func (s *Service) UpdateSettings(ctx context.Context, input UpdateSettingsInput) (*SettingsView, error) {
	if _, err := requireOrgAdmin(ctx); err != nil {
		return nil, err
	}
	if len(input.Values) == 0 {
		return nil, fmt.Errorf("%w: values are required", ErrValidation)
	}
	if len(input.Values) > 100 {
		return nil, fmt.Errorf("%w: too many settings in one update", ErrValidation)
	}
	if err := s.erp.SMTSetSettings(ctx, input.Values); err != nil {
		return nil, err
	}
	return s.Settings(ctx)
}

// ---------------------------------------------------------------------------
// Shop-floor read surfaces (delegated to the ERP SMT service).
// ---------------------------------------------------------------------------

func (s *Service) MachineContext(ctx context.Context, machineCode string) (map[string]any, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.erp.SMTMachineContext(ctx, machineCode)
}

func (s *Service) Board(ctx context.Context, lineCode string) (map[string]any, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.erp.SMTBoard(ctx, lineCode)
}

func (s *Service) WorkOrderProgress(ctx context.Context, workOrder string) (map[string]any, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.erp.SMTWorkOrderProgress(ctx, workOrder)
}

func (s *Service) TraceSN(ctx context.Context, sn string) (map[string]any, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.erp.SMTTraceSN(ctx, sn)
}

func (s *Service) ReelUnits(ctx context.Context, reelCode string) (map[string]any, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.erp.SMTReelUnits(ctx, reelCode)
}

func (s *Service) ProductionReport(ctx context.Context, from, to, lineCode string) (map[string]any, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.erp.SMTProductionReport(ctx, from, to, lineCode)
}

func (s *Service) AndonReport(ctx context.Context, from, to string) (map[string]any, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.erp.SMTAndonReport(ctx, from, to)
}

func (s *Service) DowntimeReport(ctx context.Context, from, to string) (map[string]any, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.erp.SMTDowntimeReport(ctx, from, to)
}

func (s *Service) MaterialReport(ctx context.Context, from, to string) (map[string]any, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.erp.SMTMaterialReport(ctx, from, to)
}

// ---------------------------------------------------------------------------
// Edge instance management.
// ---------------------------------------------------------------------------

func (s *Service) ListEdges(ctx context.Context) ([]EdgeInstance, error) {
	tenant, err := tenantFrom(ctx)
	if err != nil {
		return nil, err
	}
	records, err := s.repo.ListEdgeInstances(ctx, *tenant.OrganizationID)
	if err != nil {
		return nil, err
	}
	items := make([]EdgeInstance, 0, len(records))
	for index := range records {
		items = append(items, records[index].EdgeInstance)
	}
	return items, nil
}

func (s *Service) CreateEdge(ctx context.Context, input CreateEdgeInstanceInput) (*EdgeInstance, error) {
	tenant, err := requireOrgAdmin(ctx)
	if err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if input.Name == "" || input.BaseURL == "" || input.APIKey == "" {
		return nil, fmt.Errorf("%w: name, base_url and api_key are required", ErrValidation)
	}
	if err := s.validateBaseURL(ctx, input.BaseURL); err != nil {
		return nil, err
	}
	if s.box == nil {
		return nil, fmt.Errorf("%w: secretbox is not configured", ErrValidation)
	}
	apiKeyCipher, err := s.box.Encrypt(input.APIKey)
	if err != nil {
		return nil, err
	}
	secretCipher := ""
	webhookSecret := input.WebhookSecret
	if webhookSecret == "" {
		webhookSecret = uuid.NewString()
	}
	secretCipher, err = s.box.Encrypt(webhookSecret)
	if err != nil {
		return nil, err
	}
	subscribed := strings.TrimSpace(input.SubscribedEvents)
	if subscribed == "" {
		subscribed = "*"
	}
	record, err := s.repo.CreateEdgeInstance(ctx, EdgeInstanceRecord{
		EdgeInstance: EdgeInstance{
			OrganizationID:   *tenant.OrganizationID,
			Name:             input.Name,
			BaseURL:          input.BaseURL,
			SubscribedEvents: subscribed,
			Status:           EdgeInstanceStatusActive,
			AutoPush:         input.AutoPush,
		},
		APIKeyCipher:        apiKeyCipher,
		WebhookSecretCipher: secretCipher,
	})
	if err != nil {
		return nil, err
	}
	instance := record.EdgeInstance
	instance.WebhookPath = "/api/v1/smt/hooks/" + instance.InstanceKey.String()
	return &instance, nil
}

func (s *Service) UpdateEdge(ctx context.Context, key uuid.UUID, input UpdateEdgeInstanceInput) (*EdgeInstance, error) {
	tenant, err := requireOrgAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if input.Status != "" && input.Status != EdgeInstanceStatusActive && input.Status != EdgeInstanceStatusDisabled {
		return nil, fmt.Errorf("%w: status must be active or disabled", ErrValidation)
	}
	if input.BaseURL != "" {
		input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
		if err := s.validateBaseURL(ctx, input.BaseURL); err != nil {
			return nil, err
		}
	}
	var apiKeyCipher, secretCipher *string
	if input.APIKey != "" {
		encrypted, err := s.box.Encrypt(input.APIKey)
		if err != nil {
			return nil, err
		}
		apiKeyCipher = &encrypted
	}
	if input.WebhookSecret != "" {
		encrypted, err := s.box.Encrypt(input.WebhookSecret)
		if err != nil {
			return nil, err
		}
		secretCipher = &encrypted
	}
	record, err := s.repo.UpdateEdgeInstance(ctx, *tenant.OrganizationID, key, input, apiKeyCipher, secretCipher)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrNotFound
	}
	instance := record.EdgeInstance
	return &instance, nil
}

func (s *Service) validateBaseURL(ctx context.Context, baseURL string) error {
	_, err := s.outbound.Validate(ctx, baseURL)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, safeurl.ErrDisallowedDestination), errors.Is(err, safeurl.ErrUnresolvableDestination):
		return fmt.Errorf("%w: base_url must point at a reachable public host: %v", ErrValidation, err)
	default:
		return fmt.Errorf("%w: base_url must be an absolute http(s) URL: %v", ErrValidation, err)
	}
}

func (s *Service) DeleteEdge(ctx context.Context, key uuid.UUID) error {
	tenant, err := requireOrgAdmin(ctx)
	if err != nil {
		return err
	}
	return s.repo.DeleteEdgeInstance(ctx, *tenant.OrganizationID, key)
}

func (s *Service) EdgeEvents(ctx context.Context, key uuid.UUID, status string, limit int) ([]EdgeEventRecord, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.repo.ListEdgeEvents(ctx, key, strings.TrimSpace(status), limit)
}

func (s *Service) EdgeSyncLog(ctx context.Context, key uuid.UUID, limit int) ([]SyncLogRecord, error) {
	if _, err := tenantFrom(ctx); err != nil {
		return nil, err
	}
	return s.repo.ListSyncLog(ctx, key, limit)
}

// ---------------------------------------------------------------------------
// Inbound webhooks + polling.
// ---------------------------------------------------------------------------

// TenantContextForInstance builds the tenant context used to route tenant
// database access while processing edge traffic (no interactive user).
func (s *Service) TenantContextForInstance(ctx context.Context, instance *ResolvedInstance) (context.Context, error) {
	modules, err := s.repo.ListEnabledModules(ctx, instance.OrganizationID)
	if err != nil {
		return ctx, err
	}
	modules["smt_mes"] = true
	orgID := instance.OrganizationID
	tenant := &middleware.TenantContext{
		Mode:                         "saas",
		OrganizationID:               &orgID,
		AuthorityTier:                "organization_admin",
		EnabledModules:               modules,
		TenantDatabaseDeploymentMode: instance.TenantDatabaseDeploymentMode,
		TenantDatabaseName:           instance.TenantDatabaseName,
		TenantDatabaseStatus:         instance.TenantDatabaseStatus,
	}
	return context.WithValue(ctx, middleware.TenantContextKey, tenant), nil
}

// VerifySignature validates the SMT-MES webhook HMAC signature over the raw body.
func (s *Service) VerifySignature(instance *ResolvedInstance, body []byte, signature string) error {
	if s.box == nil {
		return fmt.Errorf("%w: secretbox is not configured", ErrForbidden)
	}
	if instance.WebhookSecretCipher == "" {
		return fmt.Errorf("%w: instance has no webhook secret", ErrForbidden)
	}
	secret, err := s.box.Decrypt(instance.WebhookSecretCipher)
	if err != nil {
		return fmt.Errorf("%w: cannot read webhook secret", ErrForbidden)
	}
	expect := "sha256=" + hmacHex(secret, body)
	if !hmac.Equal([]byte(expect), []byte(strings.TrimSpace(signature))) {
		return fmt.Errorf("%w: invalid webhook signature", ErrForbidden)
	}
	return nil
}

func hmacHex(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// IngestWebhook verifies and processes one raw webhook delivery.
func (s *Service) IngestWebhook(ctx context.Context, instanceKey uuid.UUID, body []byte, signature string) (string, error) {
	instance, err := s.repo.ResolveInstance(ctx, instanceKey)
	if err != nil {
		return "", err
	}
	if instance == nil {
		return "", ErrNotFound
	}
	if instance.Status != EdgeInstanceStatusActive {
		return "", fmt.Errorf("%w: instance is disabled", ErrForbidden)
	}
	if err := s.VerifySignature(instance, body, signature); err != nil {
		return "", err
	}
	var event erp.EdgeEvent
	if err := json.Unmarshal(body, &event); err != nil || event.Type == "" {
		return "", fmt.Errorf("%w: invalid event payload", ErrValidation)
	}
	return s.processEvent(ctx, instance, event, body, true)
}

var errTransient = errors.New("transient processing failure")

// processEvent deduplicates and applies one event, marking the inbox row.
func (s *Service) processEvent(ctx context.Context, instance *ResolvedInstance, event erp.EdgeEvent, raw []byte, failOnError bool) (string, error) {
	tenantCtx, err := s.TenantContextForInstance(ctx, instance)
	if err != nil {
		return "", err
	}
	rowID, inserted, err := s.repo.InsertEdgeEvent(tenantCtx, instance.InstanceKey, event.ID, event.Type, event.Ref, raw)
	if err != nil {
		return "", err
	}
	if !inserted {
		return "skipped", nil
	}
	status, applyErr := s.erp.ApplyEdgeEvent(tenantCtx, instance.InstanceKey.String(), event)
	if applyErr != nil {
		_ = s.repo.MarkEdgeEvent(tenantCtx, rowID, EdgeEventStatusFailed, applyErr.Error())
		if failOnError {
			// Let the webhook respond 5xx so SMT-MES retries with backoff.
			return EdgeEventStatusFailed, fmt.Errorf("%w: %v", errTransient, applyErr)
		}
		return EdgeEventStatusFailed, nil
	}
	if err := s.repo.MarkEdgeEvent(tenantCtx, rowID, status, ""); err != nil {
		return "", err
	}
	_ = s.repo.TouchEdgeInstanceEvent(ctx, instance.InstanceKey)
	return status, nil
}

// Pull polls the SMT-MES events API once and ingests pending events.
func (s *Service) Pull(ctx context.Context, orgID uuid.UUID, key uuid.UUID, limit int) (map[string]any, error) {
	if _, err := requireOrgAdmin(ctx); err != nil {
		return nil, err
	}
	instance, err := s.repo.ResolveInstance(ctx, key)
	if err != nil {
		return nil, err
	}
	if instance == nil || instance.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	if instance.Status != EdgeInstanceStatusActive {
		return nil, fmt.Errorf("%w: instance is disabled", ErrForbidden)
	}
	apiKey, err := s.apiKey(instance)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	url := fmt.Sprintf("%s/api/v1/integration/events?since_id=%d&limit=%d", instance.BaseURL, instance.LastPollID, limit)
	status, body, err := s.do(ctx, http.MethodGet, url, map[string]string{"X-API-Key": apiKey}, nil)
	if err != nil {
		_ = s.repo.InsertSyncLog(ctx, key, "events", "error", err.Error(), nil)
		return nil, fmt.Errorf("poll SMT-MES events: %w", err)
	}
	if status < 200 || status >= 300 {
		message := fmt.Sprintf("SMT-MES returned HTTP %d: %s", status, truncate(string(body), 200))
		_ = s.repo.InsertSyncLog(ctx, key, "events", "error", message, nil)
		return nil, fmt.Errorf("%w: %s", ErrValidation, message)
	}
	var page struct {
		Items  []json.RawMessage `json:"items"`
		LastID int64             `json:"last_id"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("%w: invalid events response: %v", ErrValidation, err)
	}
	processed, skipped, failed := 0, 0, 0
	for _, rawItem := range page.Items {
		var event erp.EdgeEvent
		if err := json.Unmarshal(rawItem, &event); err != nil || event.Type == "" {
			skipped++
			continue
		}
		result, err := s.processEvent(ctx, instance, event, rawItem, false)
		switch {
		case err != nil:
			failed++
		case result == "processed":
			processed++
		case result == EdgeEventStatusFailed:
			failed++
		default:
			skipped++
		}
	}
	if page.LastID > instance.LastPollID {
		_ = s.repo.TouchEdgeInstanceSync(ctx, key, page.LastID)
	}
	// Reload instance for the updated poll cursor.
	updated, _ := s.repo.ResolveInstance(ctx, key)
	result := map[string]any{
		"instance_key": key, "processed": processed, "skipped": skipped, "failed": failed,
	}
	if updated != nil {
		result["last_poll_id"] = updated.LastPollID
	}
	_ = s.repo.InsertSyncLog(ctx, key, "events", "ok", fmt.Sprintf("pulled %d events", len(page.Items)), result)
	return result, nil
}

func (s *Service) apiKey(instance *ResolvedInstance) (string, error) {
	if s.box == nil || instance.APIKeyCipher == "" {
		return "", fmt.Errorf("%w: instance has no API key", ErrValidation)
	}
	return s.box.Decrypt(instance.APIKeyCipher)
}

// ---------------------------------------------------------------------------
// Outbound pushes.
// ---------------------------------------------------------------------------

var pushScopes = []string{"materials", "customers", "products", "station-tables", "work-orders"}

// Push pushes selected master data / work orders to the SMT-MES instance.
func (s *Service) Push(ctx context.Context, orgID uuid.UUID, key uuid.UUID, scopes []string) (*PushResult, error) {
	if _, err := requireOrgAdmin(ctx); err != nil {
		return nil, err
	}
	instance, err := s.repo.ResolveInstance(ctx, key)
	if err != nil {
		return nil, err
	}
	if instance == nil || instance.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	if instance.Status != EdgeInstanceStatusActive {
		return nil, fmt.Errorf("%w: instance is disabled", ErrForbidden)
	}
	apiKey, err := s.apiKey(instance)
	if err != nil {
		return nil, err
	}
	if len(scopes) == 0 {
		scopes = pushScopes
	}
	result := &PushResult{InstanceKey: key, Results: []map[string]any{}}
	for _, scope := range scopes {
		payload, err := s.buildPushPayload(ctx, scope)
		if err != nil {
			result.Results = append(result.Results, map[string]any{"scope": scope, "status": "error", "message": err.Error()})
			_ = s.repo.InsertSyncLog(ctx, key, scope, "error", err.Error(), nil)
			continue
		}
		if payload == nil || len(payload) == 0 {
			result.Results = append(result.Results, map[string]any{"scope": scope, "status": "skipped", "message": "no data"})
			continue
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		url := fmt.Sprintf("%s/api/v1/integration/%s", instance.BaseURL, scope)
		statusCode, responseBody, err := s.do(ctx, http.MethodPost, url, map[string]string{"X-API-Key": apiKey}, body)
		entry := map[string]any{"scope": scope, "rows": len(payload)}
		logStatus := "ok"
		switch {
		case err != nil:
			entry["status"] = "error"
			entry["message"] = err.Error()
			logStatus = "error"
		case statusCode < 200 || statusCode >= 300:
			entry["status"] = "error"
			entry["message"] = fmt.Sprintf("HTTP %d: %s", statusCode, truncate(string(responseBody), 200))
			logStatus = "error"
		default:
			entry["status"] = "ok"
			parsed := map[string]any{}
			if json.Unmarshal(responseBody, &parsed) == nil {
				entry["response"] = parsed
			}
		}
		result.Results = append(result.Results, entry)
		_ = s.repo.InsertSyncLog(ctx, key, scope, logStatus, fmt.Sprint(entry["status"]), entry)
	}
	_ = s.repo.TouchEdgeInstanceSync(ctx, key, 0)
	return result, nil
}

func (s *Service) buildPushPayload(ctx context.Context, scope string) ([]map[string]any, error) {
	switch scope {
	case "materials":
		items, err := s.erp.SMTExportRecords(ctx, "MITM", map[string]any{}, 5000)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]any, 0, len(items))
		for _, item := range items {
			rows = append(rows, map[string]any{
				"part_no":      item["key"],
				"name":         item["ItemName"],
				"unit":         item["InvntryUom"],
				"spec":         item["FrgnName"],
				"reel_std_qty": item["reel_std_qty"],
				"safety_stock": item["safety_stock"],
				"is_active":    true,
			})
		}
		return rows, nil
	case "customers":
		partners, err := s.erp.SMTExportRecords(ctx, "MCRD", map[string]any{"CardType": "C"}, 5000)
		if err != nil {
			return nil, err
		}
		maps, err := s.erp.SMTExportRecords(ctx, "MCPM", map[string]any{}, 5000)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]any, 0, len(partners))
		for _, partner := range partners {
			code := fmt.Sprint(partner["key"])
			partMaps := []map[string]any{}
			for _, entry := range maps {
				if fmt.Sprint(entry["customer_code"]) == code {
					partMaps = append(partMaps, map[string]any{
						"customer_part_no": entry["customer_part_no"],
						"part_no":          entry["part_no"],
					})
				}
			}
			rows = append(rows, map[string]any{
				"code": code, "name": partner["CardName"], "part_maps": partMaps, "is_active": true,
			})
		}
		return rows, nil
	case "products":
		products, err := s.erp.SMTExportRecords(ctx, "MPRD", map[string]any{}, 2000)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]any, 0, len(products))
		for _, product := range products {
			code := fmt.Sprint(product["key"])
			bomRows, err := s.erp.SMTExportChildren(ctx, "MPRD", code, "PRD1")
			if err != nil {
				return nil, err
			}
			bom := make([]map[string]any, 0, len(bomRows))
			for _, line := range bomRows {
				bom = append(bom, map[string]any{
					"part_no":         line["PartNo"],
					"qty":             line["Qty"],
					"ref_designators": line["RefDesignators"],
					"alt_part_nos":    line["AltPartNos"],
				})
			}
			rows = append(rows, map[string]any{
				"product_code":  code,
				"name":          product["Name"],
				"version":       product["version"],
				"customer_code": product["customer_code"],
				"bom":           bom,
			})
		}
		return rows, nil
	case "station-tables":
		setups, err := s.erp.SMTExportRecords(ctx, "MFSB", map[string]any{}, 2000)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]any, 0, len(setups))
		for _, setup := range setups {
			code := fmt.Sprint(setup["key"])
			items, err := s.erp.SMTExportChildren(ctx, "MFSB", code, "FSB1")
			if err != nil {
				return nil, err
			}
			slotRows := make([]map[string]any, 0, len(items))
			for _, item := range items {
				slotRows = append(slotRows, map[string]any{"slot_no": item["SlotNo"], "part_no": item["PartNo"]})
			}
			rows = append(rows, map[string]any{
				"product_code": setup["product_code"],
				"machine_code": setup["machine_code"],
				"program_name": setup["program_name"],
				"version":      setup["version"],
				"items":        slotRows,
			})
		}
		return rows, nil
	case "work-orders":
		orders, err := s.erp.SMTExportRecords(ctx, "MSWO", map[string]any{}, 2000)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]any, 0, len(orders))
		for _, order := range orders {
			rows = append(rows, map[string]any{
				"wo_no":         order["key"],
				"product_code":  order["product_code"],
				"line_code":     order["line_code"],
				"plan_qty":      order["plan_qty"],
				"customer_code": order["customer_code"],
				"status":        order["Status"],
			})
		}
		return rows, nil
	default:
		return nil, fmt.Errorf("%w: unknown push scope %s", ErrValidation, scope)
	}
}

// PushWorkOrderStart asks the edge instance to start a work order.
func (s *Service) PushWorkOrderAction(ctx context.Context, orgID uuid.UUID, key uuid.UUID, workOrder, action string) error {
	if _, err := requireOrgAdmin(ctx); err != nil {
		return err
	}
	instance, err := s.repo.ResolveInstance(ctx, key)
	if err != nil {
		return err
	}
	if instance == nil || instance.OrganizationID != orgID {
		return ErrNotFound
	}
	if instance.Status != EdgeInstanceStatusActive {
		return fmt.Errorf("%w: instance is disabled", ErrForbidden)
	}
	switch action {
	case "start", "progress", "finish", "close":
	default:
		return fmt.Errorf("%w: unsupported work order action %s", ErrValidation, action)
	}
	apiKey, err := s.apiKey(instance)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/integration/work-orders/%s/%s", instance.BaseURL, workOrder, action)
	statusCode, body, err := s.do(ctx, http.MethodPost, url, map[string]string{"X-API-Key": apiKey}, []byte("{}"))
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("%w: SMT-MES returned HTTP %d: %s", ErrValidation, statusCode, truncate(string(body), 200))
	}
	_ = s.repo.InsertSyncLog(ctx, key, "work-order:"+action, "ok", workOrder, nil)
	return nil
}

func truncate(value string, size int) string {
	if len(value) <= size {
		return value
	}
	return value[:size]
}
