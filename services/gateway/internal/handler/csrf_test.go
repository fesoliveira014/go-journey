package handler_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

func csrfTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()
	set := template.Must(template.New("base.html").Parse(`TOKEN:{{.CSRFToken}}`))
	template.Must(set.New("nav").Parse(`{{define "nav"}}{{end}}`))
	template.Must(set.New("flash").Parse(`{{define "flash"}}{{end}}`))
	template.Must(set.New("content").Parse(`{{define "content"}}{{end}}`))
	return map[string]*template.Template{"login.html": set}
}

func TestRenderSetsCSRFTokenCookie(t *testing.T) {
	t.Parallel()
	srv := handler.New(nil, nil, nil, nil, csrfTemplates(t))

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	srv.LoginPage(rec, req)

	if !strings.HasPrefix(rec.Body.String(), "TOKEN:") || rec.Body.String() == "TOKEN:" {
		t.Fatalf("expected rendered CSRF token, got %q", rec.Body.String())
	}
	var csrfCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "csrf_token" {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil {
		t.Fatal("expected csrf_token cookie")
	}
	if !csrfCookie.HttpOnly {
		t.Fatal("expected csrf_token cookie to be HttpOnly")
	}
}

func TestCSRFMiddlewareRejectsMissingToken(t *testing.T) {
	t.Parallel()
	srv := handler.New(nil, nil, nil, nil, nil)
	called := false
	wrapped := srv.CSRF(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodPost, "/admin/books", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not run without CSRF token")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestCSRFMiddlewareAcceptsMatchingToken(t *testing.T) {
	t.Parallel()
	srv := handler.New(nil, nil, nil, nil, csrfTemplates(t))

	getReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	getRec := httptest.NewRecorder()
	srv.LoginPage(getRec, getReq)
	token := strings.TrimPrefix(getRec.Body.String(), "TOKEN:")
	if token == "" {
		t.Fatal("expected rendered CSRF token")
	}

	cookies := getRec.Result().Cookies()
	called := false
	wrapped := srv.CSRF(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	form := url.Values{"_csrf": {token}}
	postReq := httptest.NewRequest(http.MethodPost, "/admin/books", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		postReq.AddCookie(c)
	}
	postRec := httptest.NewRecorder()
	wrapped.ServeHTTP(postRec, postReq)

	if !called {
		t.Fatal("handler should run when CSRF token matches")
	}
	if postRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", postRec.Code)
	}
}
