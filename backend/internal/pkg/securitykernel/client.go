package securitykernel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDenied      = errors.New("security kernel denied request")
	ErrUnavailable = errors.New("security kernel unavailable")
)

type Config struct {
	URL             string
	SharedSecret    string
	EnforcementMode string
	Required        bool
}

type Client interface {
	Authorize(ctx context.Context, request Request) (Decision, error)
}

type HealthChecker interface {
	CheckHealth(context.Context) error
}

type HTTPClient struct {
	baseURL         string
	sharedSecret    string
	enforcementMode string
	httpClient      *http.Client
}

type NoopClient struct{}

type UnavailableClient struct {
	reason          string
	enforcementMode string
}

type Actor struct {
	ActorID         uuid.UUID `json:"actor_id"`
	ActorType       string    `json:"actor_type"`
	AuthorityTier   string    `json:"authority_tier"`
	IsPlatformAdmin bool      `json:"is_platform_admin"`
}

type Resource struct {
	ModuleKey             string     `json:"module_key"`
	ResourceType          string     `json:"resource_type"`
	Action                string     `json:"action"`
	ScopeLevel            string     `json:"scope_level"`
	OrganizationID        *uuid.UUID `json:"organization_id,omitempty"`
	RequiredAuthorityTier string     `json:"required_authority_tier"`
	RequiredLicenseMode   string     `json:"required_license_mode"`
}

type Request struct {
	Actor            Actor          `json:"actor"`
	OrganizationID   *uuid.UUID     `json:"organization_id,omitempty"`
	DistributionMode string         `json:"distribution_mode"`
	LicenseMode      string         `json:"license_mode"`
	EnabledModules   []string       `json:"enabled_modules"`
	EnabledFeatures  []string       `json:"enabled_features"`
	Resource         Resource       `json:"resource"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type Decision struct {
	Allowed      bool   `json:"allowed"`
	Reason       string `json:"reason"`
	DecisionType string `json:"decision_type"`
}

func NewClient(cfg Config) Client {
	mode := strings.TrimSpace(cfg.EnforcementMode)
	if mode == "" {
		mode = "blocking"
	}
	if strings.TrimSpace(cfg.URL) == "" {
		if cfg.Required {
			return UnavailableClient{reason: "security_kernel_url_required", enforcementMode: mode}
		}
		return NewNoopClient()
	}
	if strings.TrimSpace(cfg.SharedSecret) == "" {
		return UnavailableClient{reason: "security_kernel_shared_secret_required", enforcementMode: mode}
	}
	return &HTTPClient{
		baseURL:         strings.TrimRight(strings.TrimSpace(cfg.URL), "/"),
		sharedSecret:    cfg.SharedSecret,
		enforcementMode: mode,
		httpClient:      &http.Client{Timeout: 5 * time.Second},
	}
}

func ValidateConfig(cfg Config) error {
	configuredURL := strings.TrimSpace(cfg.URL)
	secret := strings.TrimSpace(cfg.SharedSecret)
	mode := strings.ToLower(strings.TrimSpace(cfg.EnforcementMode))
	if mode == "" {
		mode = "blocking"
	}
	if cfg.Required && configuredURL == "" {
		return fmt.Errorf("security kernel url is required")
	}
	if configuredURL == "" {
		return nil
	}
	parsed, err := url.Parse(configuredURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("security kernel url must be an absolute http or https URL")
	}
	if len(secret) < 32 {
		return fmt.Errorf("security kernel shared secret must contain at least 32 characters")
	}
	if cfg.Required && mode != "blocking" {
		return fmt.Errorf("required security kernel must use blocking enforcement")
	}
	if mode != "blocking" && mode != "audit" {
		return fmt.Errorf("security kernel enforcement mode must be blocking or audit")
	}
	return nil
}

func NewNoopClient() Client {
	return NoopClient{}
}

func (NoopClient) Authorize(context.Context, Request) (Decision, error) {
	return Decision{Allowed: true, Reason: "security_kernel_not_configured", DecisionType: "allow"}, nil
}

func (NoopClient) CheckHealth(context.Context) error {
	return nil
}

func (c UnavailableClient) Authorize(context.Context, Request) (Decision, error) {
	decision := deny(c.reason)
	if c.enforcementMode == "audit" {
		decision.Allowed = true
		decision.DecisionType = "allow"
		return decision, nil
	}
	return decision, fmt.Errorf("%w: %s", ErrUnavailable, c.reason)
}

func (c UnavailableClient) CheckHealth(context.Context) error {
	return fmt.Errorf("%w: %s", ErrUnavailable, c.reason)
}

func (c *HTTPClient) Authorize(ctx context.Context, request Request) (Decision, error) {
	request.applyDefaults()
	body, err := json.Marshal(request)
	if err != nil {
		return deny("invalid_security_request"), fmt.Errorf("%w: marshal request: %v", ErrDenied, err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/authorize", bytes.NewReader(body))
	if err != nil {
		return deny("invalid_security_request"), fmt.Errorf("%w: create request: %v", ErrUnavailable, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.sign(httpReq, body)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return c.unavailableDecision(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.unavailableDecision(fmt.Errorf("status %d", resp.StatusCode))
	}
	var decision Decision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		return c.unavailableDecision(err)
	}
	if !decision.Allowed {
		if decision.Reason == "" {
			decision.Reason = "denied"
		}
		return decision, fmt.Errorf("%w: %s", ErrDenied, decision.Reason)
	}
	return decision, nil
}

func (c *HTTPClient) CheckHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("%w: create health request: %v", ErrUnavailable, err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: health status %d", ErrUnavailable, resp.StatusCode)
	}
	return nil
}

func (c *HTTPClient) sign(req *http.Request, body []byte) {
	if c.sharedSecret == "" {
		return
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := uuid.NewString()
	mac := hmac.New(sha256.New, []byte(c.sharedSecret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write([]byte(nonce))
	mac.Write([]byte("."))
	mac.Write(body)
	req.Header.Set("X-Security-Timestamp", timestamp)
	req.Header.Set("X-Security-Nonce", nonce)
	req.Header.Set("X-Security-Signature", hex.EncodeToString(mac.Sum(nil)))
}

func (c *HTTPClient) unavailableDecision(err error) (Decision, error) {
	decision := deny("security_kernel_unavailable")
	if c.enforcementMode == "audit" {
		decision.Allowed = true
		decision.DecisionType = "allow"
		return decision, nil
	}
	return decision, fmt.Errorf("%w: %v", ErrUnavailable, err)
}

func (r *Request) applyDefaults() {
	if r.DistributionMode == "" {
		r.DistributionMode = "saas"
	}
	if r.LicenseMode == "" {
		r.LicenseMode = "commercial"
	}
	if r.EnabledModules == nil {
		r.EnabledModules = []string{}
	}
	if r.EnabledFeatures == nil {
		r.EnabledFeatures = []string{}
	}
	if r.Resource.RequiredAuthorityTier == "" {
		r.Resource.RequiredAuthorityTier = "executor"
	}
	if r.Resource.RequiredLicenseMode == "" {
		r.Resource.RequiredLicenseMode = "community"
	}
}

func deny(reason string) Decision {
	return Decision{Allowed: false, Reason: reason, DecisionType: "deny"}
}
