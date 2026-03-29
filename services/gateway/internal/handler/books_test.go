package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

func TestBooksHandler_ReturnsList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/books", nil)
	rec := httptest.NewRecorder()

	handler.Books(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", contentType)
	}

	var books []handler.Book
	if err := json.NewDecoder(rec.Body).Decode(&books); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(books) == 0 {
		t.Error("expected at least one book in the list")
	}

	first := books[0]
	if first.Title == "" {
		t.Error("expected book to have a title")
	}
	if first.Author == "" {
		t.Error("expected book to have an author")
	}
}

func TestBooksHandler_RejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/books", nil)
	rec := httptest.NewRecorder()

	handler.Books(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
