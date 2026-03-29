package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

func TestHealthHandler_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// json.Encode appends a trailing newline
	expected := "{\"status\":\"ok\"}\n"
	if body := rec.Body.String(); body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

func TestHealthHandler_RejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.Health(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
