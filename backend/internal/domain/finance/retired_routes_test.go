package finance

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestLegacyFinanceRoutesReturnExplicitReplacement(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(nil).RegisterRoutes(router)
	for _, path := range []string{"receivables", "receipts", "payables", "payments"} {
		for _, method := range []string{"GET", "POST", "PATCH", "DELETE"} {
			for _, suffix := range []string{"", "/legacy-id", "/legacy-id/allocate"} {
				t.Run(method+"/"+path+suffix, func(t *testing.T) {
					response := httptest.NewRecorder()
					router.ServeHTTP(response, httptest.NewRequest(method, "/finance/"+path+suffix, nil))
					if response.Code != http.StatusGone || !strings.Contains(response.Body.String(), "/ontology/objects/") {
						t.Fatalf("response = %d %s", response.Code, response.Body.String())
					}
				})
			}
		}
	}
}
