package handler_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
	"github.com/google/uuid"
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
	template.Must(bookSet.New("book_cards").Parse(
		`{{define "book_cards"}}{{range .}}CARD:{{.Title}} {{end}}{{end}}`,
	))

	// error.html: used by renderError / handleGRPCError
	errSet := template.Must(template.New("base.html").Parse(
		`ERROR:{{.Data.Status}}:{{.Data.Message}}`,
	))
	template.Must(errSet.New("book_cards").Parse(
		`{{define "book_cards"}}{{range .}}CARD:{{.Title}} {{end}}{{end}}`,
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
	srv := handler.New(nil, mock, nil, tmpl)

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
	srv := handler.New(nil, mock, nil, tmpl)

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
	srv := handler.New(nil, mock, nil, tmpl)

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
	srv := handler.New(nil, mock, nil, tmpl)

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
	srv := handler.New(nil, mock, nil, tmpl)

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
	srv := handler.New(nil, mock, nil, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/books/999", nil)
	req.SetPathValue("id", "999")
	rec := httptest.NewRecorder()

	srv.BookDetail(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// adminTestTemplates extends catalogTestTemplates with templates needed for
// admin handler tests.
func adminTestTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()
	m := catalogTestTemplates(t)

	newSet := template.Must(template.New("base.html").Parse(
		`ADMIN_NEW:{{if .Data.Error}}ERR:{{.Data.Error}}{{end}}`,
	))
	template.Must(newSet.New("book_cards").Parse(
		`{{define "book_cards"}}{{range .}}CARD:{{.Title}} {{end}}{{end}}`,
	))
	m["admin_book_new.html"] = newSet

	editSet := template.Must(template.New("base.html").Parse(
		`ADMIN_EDIT:{{.Data.Title}}:{{.Data.Author}}{{if .Data.Error}} ERR:{{.Data.Error}}{{end}}`,
	))
	template.Must(editSet.New("book_cards").Parse(
		`{{define "book_cards"}}{{range .}}CARD:{{.Title}} {{end}}{{end}}`,
	))
	m["admin_book_edit.html"] = editSet

	return m
}

// withAdmin returns a copy of r with admin user injected into the context.
func withAdmin(r *http.Request) *http.Request {
	ctx := pkgauth.ContextWithUser(r.Context(), uuid.New(), "admin")
	return r.WithContext(ctx)
}

// withMember returns a copy of r with a non-admin user injected into the context.
func withMember(r *http.Request) *http.Request {
	ctx := pkgauth.ContextWithUser(r.Context(), uuid.New(), "member")
	return r.WithContext(ctx)
}

// ---- Admin handler tests ----

func TestAdminBookNew_RequiresAdmin(t *testing.T) {
	tmpl := adminTestTemplates(t)
	srv := handler.New(nil, &mockCatalogClient{}, nil, tmpl)

	// No user in context → redirect to /login
	req := httptest.NewRequest(http.MethodGet, "/admin/books/new", nil)
	rec := httptest.NewRecorder()

	srv.AdminBookNew(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestAdminBookNew_NonAdmin(t *testing.T) {
	tmpl := adminTestTemplates(t)
	srv := handler.New(nil, &mockCatalogClient{}, nil, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/admin/books/new", nil)
	req = withMember(req)
	rec := httptest.NewRecorder()

	srv.AdminBookNew(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestAdminBookNew_Admin(t *testing.T) {
	tmpl := adminTestTemplates(t)
	srv := handler.New(nil, &mockCatalogClient{}, nil, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/admin/books/new", nil)
	req = withAdmin(req)
	rec := httptest.NewRecorder()

	srv.AdminBookNew(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ADMIN_NEW:") {
		t.Errorf("expected admin_book_new.html rendered, got %q", rec.Body.String())
	}
}

func TestAdminBookCreate_Success(t *testing.T) {
	var captured *catalogv1.CreateBookRequest
	mock := &mockCatalogClient{
		createBookFn: func(_ context.Context, in *catalogv1.CreateBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
			captured = in
			return &catalogv1.Book{Id: "99", Title: in.Title}, nil
		},
	}
	tmpl := adminTestTemplates(t)
	srv := handler.New(nil, mock, nil, tmpl)

	form := url.Values{
		"title":          {"The Pragmatic Programmer"},
		"author":         {"Andy Hunt"},
		"isbn":           {"978-0135957059"},
		"genre":          {"Programming"},
		"description":    {"A classic"},
		"published_year": {"1999"},
		"total_copies":   {"5"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/books", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withAdmin(req)
	rec := httptest.NewRecorder()

	srv.AdminBookCreate(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/books" {
		t.Errorf("expected redirect to /books, got %q", loc)
	}
	// Flash cookie must be set.
	cookies := rec.Result().Cookies()
	var flash string
	for _, c := range cookies {
		if c.Name == "flash" {
			flash = c.Value
		}
	}
	if flash != "Book created" {
		t.Errorf("expected flash 'Book created', got %q", flash)
	}
	if captured == nil {
		t.Fatal("CreateBook was not called")
	}
	if captured.Title != "The Pragmatic Programmer" {
		t.Errorf("unexpected title: %q", captured.Title)
	}
}

func TestAdminBookEdit_LoadsBook(t *testing.T) {
	mock := &mockCatalogClient{
		getBookFn: func(_ context.Context, in *catalogv1.GetBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
			if in.Id != "42" {
				return nil, status.Error(codes.NotFound, "not found")
			}
			return &catalogv1.Book{
				Id:     "42",
				Title:  "Clean Code",
				Author: "Robert Martin",
			}, nil
		},
	}
	tmpl := adminTestTemplates(t)
	srv := handler.New(nil, mock, nil, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/admin/books/42", nil)
	req.SetPathValue("id", "42")
	req = withAdmin(req)
	rec := httptest.NewRecorder()

	srv.AdminBookEdit(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Clean Code") {
		t.Errorf("expected book title in edit form, got %q", body)
	}
}

func TestAdminBookUpdate_Success(t *testing.T) {
	var capturedID string
	mock := &mockCatalogClient{
		updateBookFn: func(_ context.Context, in *catalogv1.UpdateBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
			capturedID = in.Id
			return &catalogv1.Book{Id: in.Id, Title: in.Title}, nil
		},
	}
	tmpl := adminTestTemplates(t)
	srv := handler.New(nil, mock, nil, tmpl)

	form := url.Values{
		"title":          {"Clean Code (2nd ed)"},
		"author":         {"Robert Martin"},
		"isbn":           {"978-0132350884"},
		"genre":          {"Programming"},
		"description":    {"Updated"},
		"published_year": {"2008"},
		"total_copies":   {"3"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/books/42", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "42")
	req = withAdmin(req)
	rec := httptest.NewRecorder()

	srv.AdminBookUpdate(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/books/42" {
		t.Errorf("expected redirect to /books/42, got %q", loc)
	}
	if capturedID != "42" {
		t.Errorf("expected UpdateBook called with id '42', got %q", capturedID)
	}
	cookies := rec.Result().Cookies()
	var flash string
	for _, c := range cookies {
		if c.Name == "flash" {
			flash = c.Value
		}
	}
	if flash != "Book updated" {
		t.Errorf("expected flash 'Book updated', got %q", flash)
	}
}

func TestAdminBookDelete_Success(t *testing.T) {
	var deletedID string
	mock := &mockCatalogClient{
		deleteBookFn: func(_ context.Context, in *catalogv1.DeleteBookRequest, _ ...grpc.CallOption) (*catalogv1.DeleteBookResponse, error) {
			deletedID = in.Id
			return &catalogv1.DeleteBookResponse{}, nil
		},
	}
	tmpl := adminTestTemplates(t)
	srv := handler.New(nil, mock, nil, tmpl)

	req := httptest.NewRequest(http.MethodPost, "/admin/books/42/delete", nil)
	req.SetPathValue("id", "42")
	req = withAdmin(req)
	rec := httptest.NewRecorder()

	srv.AdminBookDelete(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/books" {
		t.Errorf("expected redirect to /books, got %q", loc)
	}
	if deletedID != "42" {
		t.Errorf("expected DeleteBook called with id '42', got %q", deletedID)
	}
	cookies := rec.Result().Cookies()
	var flash string
	for _, c := range cookies {
		if c.Name == "flash" {
			flash = c.Value
		}
	}
	if flash != "Book deleted" {
		t.Errorf("expected flash 'Book deleted', got %q", flash)
	}
}
