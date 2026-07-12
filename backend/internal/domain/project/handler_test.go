package project

import (
	"fmt"
	"net/http"
	"testing"
)

func TestStatusFromErrorMapsUnavailableToServiceUnavailable(t *testing.T) {
	if got := statusFromError(fmt.Errorf("%w: no active AI model", ErrUnavailable)); got != http.StatusServiceUnavailable {
		t.Fatalf("statusFromError() = %d, want %d", got, http.StatusServiceUnavailable)
	}
}
