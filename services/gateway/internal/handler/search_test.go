package handler_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"

	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

type mockSearchClient struct {
	searchv1.SearchServiceClient
	searchResp  *searchv1.SearchResponse
	suggestResp *searchv1.SuggestResponse
	err         error
}

func (m *mockSearchClient) Search(_ context.Context, _ *searchv1.SearchRequest, _ ...grpc.CallOption) (*searchv1.SearchResponse, error) {
	return m.searchResp, m.err
}

func (m *mockSearchClient) Suggest(_ context.Context, _ *searchv1.SuggestRequest, _ ...grpc.CallOption) (*searchv1.SuggestResponse, error) {
	return m.suggestResp, m.err
}

// searchTestTemplates returns a template map with entries needed for search handler tests.
// Every entry in the map includes the suggestions.html named template so that
// whichever entry becomes baseTmpl (map iteration order is random), renderPartial
// can always execute "suggestions.html".
func searchTestTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()

	suggestTmpl := `{{define "suggestions.html"}}{{range .}}<a>{{.Title}}</a>{{end}}{{end}}`

	// search.html: minimal template that renders HasResults flag.
	searchSet := template.Must(template.New("base.html").Parse(
		`{{if .Data.HasResults}}RESULTS:{{.Data.TotalHits}}{{else}}EMPTY{{end}}`,
	))
	template.Must(searchSet.New("suggestions.html").Parse(suggestTmpl))

	// error.html: used by handleGRPCError.
	errSet := template.Must(template.New("base.html").Parse(
		`ERROR:{{.Data.Status}}:{{.Data.Message}}`,
	))
	template.Must(errSet.New("suggestions.html").Parse(suggestTmpl))

	return map[string]*template.Template{
		"search.html": searchSet,
		"error.html":  errSet,
	}
}

func TestSearchPage_Success(t *testing.T) {
	mock := &mockSearchClient{
		searchResp: &searchv1.SearchResponse{
			Books: []*searchv1.BookResult{
				{Id: "1", Title: "Go Book", Author: "Author"},
			},
			TotalHits:   1,
			QueryTimeMs: 2,
		},
	}
	tmpl := searchTestTemplates(t)
	srv := handler.New(nil, nil, nil, mock, tmpl)

	req := httptest.NewRequest("GET", "/search?q=go", nil)
	w := httptest.NewRecorder()
	srv.SearchPage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html, got %s", ct)
	}
}

func TestSearchPage_EmptyQuery(t *testing.T) {
	tmpl := searchTestTemplates(t)
	srv := handler.New(nil, nil, nil, nil, tmpl)

	req := httptest.NewRequest("GET", "/search", nil)
	w := httptest.NewRecorder()
	srv.SearchPage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for empty query (shows empty form), got %d", w.Code)
	}
}

func TestSearchSuggest_Success(t *testing.T) {
	mock := &mockSearchClient{
		suggestResp: &searchv1.SuggestResponse{
			Suggestions: []*searchv1.Suggestion{
				{BookId: "1", Title: "Go in Action", Author: "Kennedy"},
			},
		},
	}
	tmpl := searchTestTemplates(t)
	srv := handler.New(nil, nil, nil, mock, tmpl)

	req := httptest.NewRequest("GET", "/search/suggest?prefix=go", nil)
	w := httptest.NewRecorder()
	srv.SearchSuggest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSearchSuggest_ShortPrefix(t *testing.T) {
	tmpl := searchTestTemplates(t)
	srv := handler.New(nil, nil, nil, nil, tmpl)

	req := httptest.NewRequest("GET", "/search/suggest?prefix=g", nil)
	w := httptest.NewRecorder()
	srv.SearchSuggest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (empty partial), got %d", w.Code)
	}
}
