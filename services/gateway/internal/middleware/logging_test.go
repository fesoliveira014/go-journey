package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fesoliveira014/library-system/services/gateway/internal/middleware"
)

func TestLogging_CapturesStatus(t *testing.T) {
	const wantStatus = http.StatusTeapot

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(wantStatus)
	})

	handler := middleware.Logging(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != wantStatus {
		t.Errorf("expected status %d, got %d", wantStatus, rec.Code)
	}
}

func TestLogging_DefaultStatus200(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write body without calling WriteHeader explicitly.
		w.Write([]byte("hello")) //nolint:errcheck
	})

	handler := middleware.Logging(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
