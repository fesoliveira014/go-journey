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
	t.Parallel()
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
	t.Parallel()
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

func TestFlash_RoundTrip(t *testing.T) {
	t.Parallel()
	srv := handler.New(nil, nil, nil, nil, testTemplates(t))

	// Set a flash on the first response.
	setRec := httptest.NewRecorder()
	srv.ExportedSetFlash(setRec, "saved!")

	// Replay the set cookie on the next request.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range setRec.Result().Cookies() {
		if c.Name == "flash" {
			req.AddCookie(c)
		}
	}
	rec := httptest.NewRecorder()

	msg := srv.ExportedConsumeFlash(rec, req)
	if msg != "saved!" {
		t.Errorf("expected flash %q, got %q", "saved!", msg)
	}

	// The response should set a clearing cookie (MaxAge: -1).
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "flash" && c.MaxAge == -1 {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Error("expected a flash cookie with MaxAge=-1 to clear the cookie")
	}
}

func TestConsumeFlash_RejectsTamperedCookie(t *testing.T) {
	t.Parallel()
	srv := handler.New(nil, nil, nil, nil, testTemplates(t))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "flash", Value: "not-a-signed-value"})
	rec := httptest.NewRecorder()

	msg := srv.ExportedConsumeFlash(rec, req)
	if msg != "" {
		t.Errorf("expected empty flash for tampered cookie, got %q", msg)
	}
}

func TestConsumeFlash_RejectsCrossServerCookie(t *testing.T) {
	t.Parallel()
	// A cookie minted by one server must not decode under another — this is
	// the HMAC guarantee we rely on.
	srvA := handler.New(nil, nil, nil, nil, testTemplates(t))
	srvB := handler.New(nil, nil, nil, nil, testTemplates(t))

	mint := httptest.NewRecorder()
	srvA.ExportedSetFlash(mint, "secret from A")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range mint.Result().Cookies() {
		if c.Name == "flash" {
			req.AddCookie(c)
		}
	}
	rec := httptest.NewRecorder()

	msg := srvB.ExportedConsumeFlash(rec, req)
	if msg != "" {
		t.Errorf("expected cookie signed by srvA to be rejected by srvB, got %q", msg)
	}
}

func TestConsumeFlash_ReturnsEmptyWhenNoCookie(t *testing.T) {
	t.Parallel()
	srv := handler.New(nil, nil, nil, nil, testTemplates(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	msg := srv.ExportedConsumeFlash(rec, req)
	if msg != "" {
		t.Errorf("expected empty flash, got %q", msg)
	}
}
