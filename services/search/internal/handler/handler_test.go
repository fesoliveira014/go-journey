package handler_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/handler"
	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

type mockService struct {
	searchDocs  []model.BookDocument
	suggestions []model.Suggestion
	totalHits   int64
	queryTimeMs int64
}

func (m *mockService) Search(_ context.Context, _ string, _ index.SearchFilters, _, _ int) ([]model.BookDocument, int64, int64, error) {
	return m.searchDocs, m.totalHits, m.queryTimeMs, nil
}

func (m *mockService) Suggest(_ context.Context, _ string, _ int) ([]model.Suggestion, error) {
	return m.suggestions, nil
}

func TestSearchHandler_Search_Success(t *testing.T) {
	svc := &mockService{
		searchDocs:  []model.BookDocument{{ID: "1", Title: "Go Book", Author: "Author"}},
		totalHits:   1,
		queryTimeMs: 2,
	}
	h := handler.NewSearchHandler(svc)

	resp, err := h.Search(context.Background(), &searchv1.SearchRequest{
		Query:    "go",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Books) != 1 {
		t.Errorf("expected 1 book, got %d", len(resp.Books))
	}
	if resp.TotalHits != 1 {
		t.Errorf("expected total_hits 1, got %d", resp.TotalHits)
	}
	if resp.Books[0].Title != "Go Book" {
		t.Errorf("expected title 'Go Book', got %s", resp.Books[0].Title)
	}
}

func TestSearchHandler_Search_EmptyQuery(t *testing.T) {
	h := handler.NewSearchHandler(&mockService{})

	_, err := h.Search(context.Background(), &searchv1.SearchRequest{Query: ""})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestSearchHandler_Suggest_Success(t *testing.T) {
	svc := &mockService{
		suggestions: []model.Suggestion{{BookID: "1", Title: "Go in Action", Author: "Kennedy"}},
	}
	h := handler.NewSearchHandler(svc)

	resp, err := h.Suggest(context.Background(), &searchv1.SuggestRequest{
		Prefix: "go",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(resp.Suggestions))
	}
	if resp.Suggestions[0].Title != "Go in Action" {
		t.Errorf("expected 'Go in Action', got %s", resp.Suggestions[0].Title)
	}
}

func TestSearchHandler_Suggest_EmptyPrefix(t *testing.T) {
	h := handler.NewSearchHandler(&mockService{})

	_, err := h.Suggest(context.Background(), &searchv1.SuggestRequest{Prefix: ""})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty prefix, got %v", st.Code())
	}
}
