package handler_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

// testTemplates builds a minimal map[string]*template.Template sufficient for
// render() tests. Each entry simulates a cloned base+page set: "base.html" is
// the named template that render() executes, and it delegates to page-specific
// defines via {{template}}.
func testTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()

	// "page.html" entry: base.html renders TITLE:<data>, no nav/flash needed.
	pageSet := template.Must(template.New("base.html").Parse(`TITLE:{{.Data}}`))
	template.Must(pageSet.New("nav").Parse(`{{define "nav"}}{{end}}`))
	template.Must(pageSet.New("flash").Parse(`{{define "flash"}}{{end}}`))
	template.Must(pageSet.New("content").Parse(`{{define "content"}}{{end}}`))

	// "error.html" entry: base.html renders ERROR:<status>:<message>.
	errSet := template.Must(template.New("base.html").Parse(`ERROR:{{.Data.Status}}:{{.Data.Message}}`))
	template.Must(errSet.New("nav").Parse(`{{define "nav"}}{{end}}`))
	template.Must(errSet.New("flash").Parse(`{{define "flash"}}{{end}}`))
	template.Must(errSet.New("content").Parse(`{{define "content"}}{{end}}`))

	return map[string]*template.Template{
		"page.html":  pageSet,
		"error.html": errSet,
	}
}

func TestRender_WritesTemplateOutput(t *testing.T) {
	tmpl := testTemplates(t)
	srv := handler.New(nil, nil, nil, nil, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.ExportedRender(rec, req, "page.html", "hello")

	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("unexpected Content-Type: %q", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, "TITLE:hello") {
		t.Errorf("expected body to contain %q, got %q", "TITLE:hello", body)
	}
}

func TestRenderError_SetsStatusCode(t *testing.T) {
	tmpl := testTemplates(t)
	srv := handler.New(nil, nil, nil, nil, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.ExportedRenderError(rec, req, http.StatusNotFound, "not found")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "ERROR:404:not found") {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestConsumeFlash_ReadsAndClearsFlashCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "flash", Value: "saved!"})
	rec := httptest.NewRecorder()

	msg := handler.ExportedConsumeFlash(rec, req)

	if msg != "saved!" {
		t.Errorf("expected flash %q, got %q", "saved!", msg)
	}

	// The response should set a clearing cookie (MaxAge: -1)
	cookies := rec.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "flash" && c.MaxAge == -1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a flash cookie with MaxAge=-1 to clear the cookie")
	}
}

func TestConsumeFlash_ReturnsEmptyWhenNoCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	msg := handler.ExportedConsumeFlash(rec, req)
	if msg != "" {
		t.Errorf("expected empty flash, got %q", msg)
	}
}
