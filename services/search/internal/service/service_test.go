package service_test

import (
	"context"
	"testing"

	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
	"github.com/fesoliveira014/library-system/services/search/internal/service"
)

type mockIndex struct {
	docs      map[string]model.BookDocument
	ensured   bool
	searchRes []model.BookDocument
}

func newMockIndex() *mockIndex {
	return &mockIndex{docs: make(map[string]model.BookDocument)}
}

func (m *mockIndex) Upsert(_ context.Context, doc model.BookDocument) error {
	m.docs[doc.ID] = doc
	return nil
}

func (m *mockIndex) Delete(_ context.Context, id string) error {
	delete(m.docs, id)
	return nil
}

func (m *mockIndex) Search(_ context.Context, query string, filters index.SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error) {
	return m.searchRes, int64(len(m.searchRes)), 1, nil
}

func (m *mockIndex) Suggest(_ context.Context, prefix string, limit int) ([]model.BookDocument, error) {
	return m.searchRes, nil
}

func (m *mockIndex) Count(_ context.Context) (int64, error) {
	return int64(len(m.docs)), nil
}

func (m *mockIndex) EnsureIndex(_ context.Context) error {
	m.ensured = true
	return nil
}

func TestSearchService_Upsert(t *testing.T) {
	idx := newMockIndex()
	svc := service.NewSearchService(idx)

	err := svc.Upsert(context.Background(), model.BookDocument{ID: "1", Title: "Go Book"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.docs) != 1 {
		t.Errorf("expected 1 doc, got %d", len(idx.docs))
	}
}

func TestSearchService_Delete(t *testing.T) {
	idx := newMockIndex()
	idx.docs["1"] = model.BookDocument{ID: "1"}
	svc := service.NewSearchService(idx)

	err := svc.Delete(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx.docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(idx.docs))
	}
}

func TestSearchService_Search_DefaultPagination(t *testing.T) {
	idx := newMockIndex()
	idx.searchRes = []model.BookDocument{{ID: "1", Title: "Go Book"}}
	svc := service.NewSearchService(idx)

	docs, total, _, err := svc.Search(context.Background(), "go", index.SearchFilters{}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("expected 1 result, got %d", len(docs))
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestSearchService_Suggest(t *testing.T) {
	idx := newMockIndex()
	idx.searchRes = []model.BookDocument{{ID: "1", Title: "Go in Action", Author: "Kennedy"}}
	svc := service.NewSearchService(idx)

	suggestions, err := svc.Suggest(context.Background(), "go", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Title != "Go in Action" {
		t.Errorf("expected title 'Go in Action', got %s", suggestions[0].Title)
	}
}

func TestSearchService_EnsureIndex(t *testing.T) {
	idx := newMockIndex()
	svc := service.NewSearchService(idx)

	err := svc.EnsureIndex(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !idx.ensured {
		t.Error("expected EnsureIndex to be called")
	}
}

func TestSearchService_Count(t *testing.T) {
	idx := newMockIndex()
	idx.docs["1"] = model.BookDocument{ID: "1"}
	svc := service.NewSearchService(idx)

	count, err := svc.Count(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}
