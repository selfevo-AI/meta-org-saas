package documentimport

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/ontology"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func validDraft() Draft {
	return Draft{Key: "IMPORT-1", Properties: map[string]any{"partner": "SUP", "date": "2026-09-15", "currency": "CNY"},
		Lines: []map[string]any{{"item": "ITEM", "warehouse": "MAIN", "quantity": "10", "unit_price": "12.5", "tax_rate": "13"}}}
}

func TestNormalizeDraftRejectsUnsafeOrInconsistentBusinessData(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Draft)
		path   string
	}{
		{"approval injection", func(d *Draft) { d.Properties["Posted"] = "Y" }, "properties.Posted"},
		{"unknown line", func(d *Draft) { d.Lines[0]["provenance"] = map[string]any{"approved": true} }, "lines.0.provenance"},
		{"negative quantity", func(d *Draft) { d.Lines[0]["quantity"] = "-1" }, "lines.0.quantity"},
		{"zero quantity", func(d *Draft) { d.Lines[0]["quantity"] = "0" }, "lines.0.quantity"},
		{"not finite", func(d *Draft) { d.Lines[0]["unit_price"] = "NaN" }, "lines.0.unit_price"},
		{"decimal precision", func(d *Draft) { d.Lines[0]["unit_price"] = "1.0000001" }, "lines.0.unit_price"},
		{"huge exponent", func(d *Draft) { d.Lines[0]["unit_price"] = "1e999999" }, "lines.0.unit_price"},
		{"computed total overflow", func(d *Draft) {
			d.Lines[0]["quantity"], d.Lines[0]["unit_price"], d.Lines[0]["tax_rate"] = "1000000000", "1000000000", "0"
		}, "properties.total"},
		{"missing reference", func(d *Draft) { delete(d.Lines[0], "warehouse") }, "lines.0.warehouse"},
		{"amount mismatch", func(d *Draft) { d.Properties["total"] = "1" }, "properties.total"},
		{"invalid date", func(d *Draft) { d.Properties["date"] = "2026-02-30" }, "properties.date"},
		{"date range", func(d *Draft) { d.Properties["due_date"] = "2026-09-14" }, "properties.due_date"},
		{"invalid currency", func(d *Draft) { d.Properties["currency"] = "US D" }, "properties.currency"},
	} {
		t.Run(test.name, func(t *testing.T) {
			draft := validDraft()
			test.change(&draft)
			_, err := normalizeDraft(draft, "purchase_order", true)
			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("error=%v", err)
			}
			for _, issue := range validation.Issues {
				if issue.Path == test.path {
					return
				}
			}
			t.Fatalf("missing issue %s: %#v", test.path, validation.Issues)
		})
	}
	result, err := normalizeDraft(validDraft(), "purchase_order", true)
	if err != nil || result.Properties["total"] != "141.25" || result.Properties["tax"] != "16.25" {
		t.Fatalf("totals=%#v err=%v", result, err)
	}
	if _, err := normalizeDraft(Draft{Key: "PARTIAL"}, "purchase_order", false); err != nil {
		t.Fatalf("partial review: %v", err)
	}
	if _, err := normalizeDraft(validDraft(), "outgoing_payment", true); err == nil {
		t.Fatal("payment allocation lines were accepted")
	}
}

func TestFilesPreserveOriginalsAndUseContentIdentity(t *testing.T) {
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	imageBytes := imageData.Bytes()
	files, hash, err := validateFiles([]SourceFile{{Name: "folder/invoice.png", Content: imageBytes}, {Name: "notes.txt", Content: []byte("invoice evidence")}})
	if err != nil {
		t.Fatal(err)
	}
	_, reversed, err := validateFiles([]SourceFile{{Name: "renamed.txt", Content: []byte("invoice evidence")}, {Name: "renamed.png", Content: imageBytes}})
	if err != nil || hash != reversed || files[0].Name != "invoice.png" || !bytes.Equal(files[0].Content, imageBytes) {
		t.Fatal("source identity changed with file order or name")
	}
	for _, source := range []SourceFile{{Name: "bad.png", Content: []byte("not an image")}, {Name: "binary.txt", Content: []byte{0, 255}}, {Name: "bad.docx", Content: []byte("not a ZIP")}, {Name: "script.svg", Content: []byte("<svg></svg>")}} {
		if _, _, err := validateFiles([]SourceFile{source}); err == nil {
			t.Fatalf("invalid file accepted: %s", source.Name)
		}
	}
	if _, _, err := validateFiles([]SourceFile{files[0], files[0]}); err == nil {
		t.Fatal("duplicate evidence accepted")
	}
}

func TestDocxAndCSVUseStructuredParsers(t *testing.T) {
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	file, _ := archive.Create("word/document.xml")
	_, _ = file.Write([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Supplier &amp; Co</w:t></w:r></w:p></w:body></w:document>`))
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	text, err := extractDocx(data.Bytes())
	if err != nil || !strings.Contains(text, "Supplier & Co") {
		t.Fatalf("DOCX=%q %v", text, err)
	}
	files, _, err := validateFiles([]SourceFile{{Name: "invoice.csv", Content: []byte("partner,date,currency,item,warehouse,quantity,unit_price,tax_rate,description\nSUP,2026-09-15,CNY,ITEM,MAIN,10,12.5,13,\"Part, blue\"\n")}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewGatewayRecognizer(nil, RecognitionConfig{}).Recognize(context.Background(), ontology.ObjectType{Key: "purchase_order"}, files)
	if err != nil || result.Method != "structured_csv" || result.Extraction.Draft.Lines[0]["description"] != "Part, blue" {
		t.Fatalf("CSV: %#v %v", result, err)
	}
	if result.Extraction.Evidence[0].SourceID != files[0].ID {
		t.Fatal("CSV evidence lost source identity")
	}
}

type captureAI struct {
	input   aigateway.InvokeInput
	content string
}

func (a *captureAI) Invoke(_ context.Context, input aigateway.InvokeInput) (*aigateway.InvokeOutput, error) {
	a.input = input
	return &aigateway.InvokeOutput{InvocationID: uuid.New(), Content: a.content}, nil
}

func TestRecognizerBindsEvidenceAndNeverRequestsTools(t *testing.T) {
	file := SourceFile{ID: uuid.New(), Name: "invoice.pdf", MediaType: "application/pdf", Content: []byte("%PDF-1.4")}
	extraction := Extraction{Draft: validDraft(), Evidence: []Evidence{{Path: "properties.partner", SourceID: file.ID, Quote: "SUP", Confidence: .7}}, Warnings: []string{}}
	encoded, _ := json.Marshal(extraction)
	ai := &captureAI{content: string(encoded)}
	result, err := NewGatewayRecognizer(ai, RecognitionConfig{ProviderType: "openai", Model: "test-vision"}).Recognize(context.Background(), ontology.ObjectType{Key: "purchase_order"}, []SourceFile{file})
	if err != nil || result.InvocationID == nil || len(ai.input.Tools) != 0 || len(ai.input.Messages[1].Attachments) != 1 || !strings.Contains(ai.input.Messages[0].Content, "untrusted") {
		t.Fatalf("recognizer: %#v %v", result, err)
	}
	for _, content := range []string{string(encoded) + ` {}`, strings.Replace(string(encoded), file.ID.String(), uuid.NewString(), 1), `{"draft":{"properties":{},"lines":[]},"evidence":[],"warnings":[],"execute":"post"}`} {
		if _, err := parseExtraction(content, []SourceFile{file}); err == nil {
			t.Fatal("untrusted extraction contract accepted")
		}
	}
}

func TestImportHTTPRequiresExplicitHumanConfirmation(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(nil, nil, nil, nil)).RegisterRoutes(router)
	for _, body := range []string{``, `{}`, `{"actor_id":"spoof","version":1}`, `{} {}`} {
		request := httptest.NewRequest(http.MethodPost, "/document-imports/"+uuid.NewString()+"/confirm", strings.NewReader(body))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusForbidden {
			t.Fatalf("body %q accepted: %d", body, response.Code)
		}
	}
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, middleware.AuthenticatedUser{ID: uuid.NewString(), Type: "agent"})
	if _, err := humanActor(ctx); !errors.Is(err, erp.ErrForbidden) {
		t.Fatal("agent impersonated a human")
	}
}
