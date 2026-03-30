package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/gateway/internal/middleware"
	"github.com/google/uuid"
)

const testSecret = "test-secret"

// captureCtxHandler is a downstream handler that records the context values it sees.
type captureCtxHandler struct {
	userID uuid.UUID
	role   string
	called bool
}

func (h *captureCtxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	h.userID, _ = pkgauth.UserIDFromContext(r.Context())
	h.role, _ = pkgauth.RoleFromContext(r.Context())
}

func TestAuth_ValidCookie(t *testing.T) {
	userID := uuid.New()
	token, err := pkgauth.GenerateToken(userID, "member", testSecret, time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	downstream := &captureCtxHandler{}
	handler := middleware.Auth(downstream, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !downstream.called {
		t.Fatal("downstream handler was not called")
	}
	if downstream.userID != userID {
		t.Errorf("UserIDFromContext: got %v, want %v", downstream.userID, userID)
	}
	if downstream.role != "member" {
		t.Errorf("RoleFromContext: got %q, want %q", downstream.role, "member")
	}
}

func TestAuth_NoCookie(t *testing.T) {
	downstream := &captureCtxHandler{}
	handler := middleware.Auth(downstream, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !downstream.called {
		t.Fatal("downstream handler was not called")
	}
	if downstream.userID != uuid.Nil {
		t.Errorf("expected uuid.Nil when no cookie, got %v", downstream.userID)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	downstream := &captureCtxHandler{}
	handler := middleware.Auth(downstream, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "this.is.garbage"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !downstream.called {
		t.Fatal("downstream handler was not called")
	}
	if downstream.userID != uuid.Nil {
		t.Errorf("expected uuid.Nil for invalid token, got %v", downstream.userID)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	userID := uuid.New()
	// Negative duration puts ExpiresAt in the past.
	token, err := pkgauth.GenerateToken(userID, "member", testSecret, -time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	downstream := &captureCtxHandler{}
	handler := middleware.Auth(downstream, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !downstream.called {
		t.Fatal("downstream handler was not called")
	}
	if downstream.userID != uuid.Nil {
		t.Errorf("expected uuid.Nil for expired token, got %v", downstream.userID)
	}
}
