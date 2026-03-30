package handler_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockCatalogClient implements catalogv1.CatalogServiceClient for testing.
type mockCatalogClient struct {
	listBooksFn          func(ctx context.Context, in *catalogv1.ListBooksRequest, opts ...grpc.CallOption) (*catalogv1.ListBooksResponse, error)
	getBookFn            func(ctx context.Context, in *catalogv1.GetBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error)
	createBookFn         func(ctx context.Context, in *catalogv1.CreateBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error)
	updateBookFn         func(ctx context.Context, in *catalogv1.UpdateBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error)
	deleteBookFn         func(ctx context.Context, in *catalogv1.DeleteBookRequest, opts ...grpc.CallOption) (*catalogv1.DeleteBookResponse, error)
	updateAvailabilityFn func(ctx context.Context, in *catalogv1.UpdateAvailabilityRequest, opts ...grpc.CallOption) (*catalogv1.UpdateAvailabilityResponse, error)
}

func (m *mockCatalogClient) ListBooks(ctx context.Context, in *catalogv1.ListBooksRequest, opts ...grpc.CallOption) (*catalogv1.ListBooksResponse, error) {
	if m.listBooksFn != nil {
		return m.listBooksFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockCatalogClient) GetBook(ctx context.Context, in *catalogv1.GetBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error) {
	if m.getBookFn != nil {
		return m.getBookFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockCatalogClient) CreateBook(ctx context.Context, in *catalogv1.CreateBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error) {
	if m.createBookFn != nil {
		return m.createBookFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockCatalogClient) UpdateBook(ctx context.Context, in *catalogv1.UpdateBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error) {
	if m.updateBookFn != nil {
		return m.updateBookFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockCatalogClient) DeleteBook(ctx context.Context, in *catalogv1.DeleteBookRequest, opts ...grpc.CallOption) (*catalogv1.DeleteBookResponse, error) {
	if m.deleteBookFn != nil {
		return m.deleteBookFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockCatalogClient) UpdateAvailability(ctx context.Context, in *catalogv1.UpdateAvailabilityRequest, opts ...grpc.CallOption) (*catalogv1.UpdateAvailabilityResponse, error) {
	if m.updateAvailabilityFn != nil {
		return m.updateAvailabilityFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// catalogTestTemplates builds a template map for catalog handler tests.
//
// The base.html template just renders the content block so we can inspect
// book titles and other content in assertions. The book_cards partial is
// included in the set so renderPartial can use it for HTMX responses.
func catalogTestTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()

	// catalog.html: renders book titles for assertions
	catalogSet := template.Must(template.New("base.html").Parse(
		`{{range .Data.Books}}BOOK:{{.Title}} {{end}}`,
	))
	template.Must(catalogSet.New("book_cards").Parse(
		`{{define "book_cards"}}{{range .}}CARD:{{.Title}} {{end}}{{end}}`,
	))

	// book.html: renders book detail fields
	bookSet := template.Must(template.New("base.html").Parse(
		`DETAIL:{{.Data.Title}}:{{.Data.Author}}`,
	))

	// error.html: used by renderError / handleGRPCError
	errSet := template.Must(template.New("base.html").Parse(
		`ERROR:{{.Data.Status}}:{{.Data.Message}}`,
	))

	return map[string]*template.Template{
		"catalog.html": catalogSet,
		"book.html":    bookSet,
		"error.html":   errSet,
	}
}

// ---- Tests ----

func TestBookList_RendersBooks(t *testing.T) {
	mock := &mockCatalogClient{
		listBooksFn: func(_ context.Context, _ *catalogv1.ListBooksRequest, _ ...grpc.CallOption) (*catalogv1.ListBooksResponse, error) {
			return &catalogv1.ListBooksResponse{
				Books: []*catalogv1.Book{
					{Id: "1", Title: "Clean Code", Author: "Robert Martin"},
					{Id: "2", Title: "DDIA", Author: "Martin Kleppmann"},
				},
			}, nil
		},
	}
	tmpl := catalogTestTemplates(t)
	srv := handler.New(nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/books", nil)
	rec := httptest.NewRecorder()

	srv.BookList(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Clean Code") {
		t.Errorf("expected body to contain 'Clean Code', got %q", body)
	}
	if !strings.Contains(body, "DDIA") {
		t.Errorf("expected body to contain 'DDIA', got %q", body)
	}
}

func TestBookList_HTMXRequest(t *testing.T) {
	mock := &mockCatalogClient{
		listBooksFn: func(_ context.Context, _ *catalogv1.ListBooksRequest, _ ...grpc.CallOption) (*catalogv1.ListBooksResponse, error) {
			return &catalogv1.ListBooksResponse{
				Books: []*catalogv1.Book{
					{Id: "3", Title: "The Go Programming Language"},
				},
			}, nil
		},
	}
	tmpl := catalogTestTemplates(t)
	srv := handler.New(nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/books", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	srv.BookList(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	// Partial response must NOT be a full HTML page.
	if strings.Contains(body, "<html") {
		t.Errorf("expected partial response (no <html> tag), got %q", body)
	}
	if !strings.Contains(body, "The Go Programming Language") {
		t.Errorf("expected book title in partial, got %q", body)
	}
}

func TestBookList_GenreFilter(t *testing.T) {
	var capturedGenre string
	mock := &mockCatalogClient{
		listBooksFn: func(_ context.Context, in *catalogv1.ListBooksRequest, _ ...grpc.CallOption) (*catalogv1.ListBooksResponse, error) {
			capturedGenre = in.Genre
			return &catalogv1.ListBooksResponse{Books: nil}, nil
		},
	}
	tmpl := catalogTestTemplates(t)
	srv := handler.New(nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/books?genre=Programming", nil)
	rec := httptest.NewRecorder()

	srv.BookList(rec, req)

	if capturedGenre != "Programming" {
		t.Errorf("expected genre 'Programming' passed to ListBooks, got %q", capturedGenre)
	}
}

func TestBookList_GRPCError(t *testing.T) {
	mock := &mockCatalogClient{
		listBooksFn: func(_ context.Context, _ *catalogv1.ListBooksRequest, _ ...grpc.CallOption) (*catalogv1.ListBooksResponse, error) {
			return nil, status.Error(codes.NotFound, "no books")
		},
	}
	tmpl := catalogTestTemplates(t)
	srv := handler.New(nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/books", nil)
	rec := httptest.NewRecorder()

	srv.BookList(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ERROR:") {
		t.Errorf("expected error body, got %q", rec.Body.String())
	}
}

func TestBookDetail_Success(t *testing.T) {
	mock := &mockCatalogClient{
		getBookFn: func(_ context.Context, in *catalogv1.GetBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
			if in.Id != "42" {
				return nil, status.Error(codes.NotFound, "not found")
			}
			return &catalogv1.Book{
				Id:     "42",
				Title:  "Designing Data-Intensive Applications",
				Author: "Martin Kleppmann",
			}, nil
		},
	}
	tmpl := catalogTestTemplates(t)
	srv := handler.New(nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/books/42", nil)
	// Simulate PathValue by using a mux that supports it; here we set it manually
	// via a wrapper. httptest doesn't call mux routing, so we set path value
	// directly on the request using r.SetPathValue (Go 1.22+).
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	srv.BookDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Designing Data-Intensive Applications") {
		t.Errorf("expected book title in detail page, got %q", body)
	}
}

func TestBookDetail_NotFound(t *testing.T) {
	mock := &mockCatalogClient{
		getBookFn: func(_ context.Context, _ *catalogv1.GetBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
			return nil, status.Error(codes.NotFound, "book not found")
		},
	}
	tmpl := catalogTestTemplates(t)
	srv := handler.New(nil, mock, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/books/999", nil)
	req.SetPathValue("id", "999")
	rec := httptest.NewRecorder()

	srv.BookDetail(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
