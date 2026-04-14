package bootstrap_test

import (
	"context"
	"testing"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/bootstrap"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
	"google.golang.org/grpc"
)

type mockSearchService struct {
	ensured  bool
	count    int64
	upserted []model.BookDocument
}

func (m *mockSearchService) EnsureIndex(_ context.Context) error {
	m.ensured = true
	return nil
}

func (m *mockSearchService) Count(_ context.Context) (int64, error) {
	return m.count, nil
}

func (m *mockSearchService) Upsert(_ context.Context, doc model.BookDocument) error {
	m.upserted = append(m.upserted, doc)
	return nil
}

type mockCatalogClient struct {
	catalogv1.CatalogServiceClient
	books []*catalogv1.Book
}

func (m *mockCatalogClient) ListBooks(_ context.Context, req *catalogv1.ListBooksRequest, _ ...grpc.CallOption) (*catalogv1.ListBooksResponse, error) {
	return &catalogv1.ListBooksResponse{Books: m.books, TotalCount: int32(len(m.books))}, nil
}

func TestBootstrap_SkipsWhenIndexHasDocuments(t *testing.T) {
	t.Parallel()
	svc := &mockSearchService{count: 5}
	catalog := &mockCatalogClient{}

	err := bootstrap.Run(context.Background(), catalog, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.ensured {
		t.Error("expected EnsureIndex to be called")
	}
	if len(svc.upserted) != 0 {
		t.Errorf("expected no upserts when index is populated, got %d", len(svc.upserted))
	}
}

func TestBootstrap_IndexesAllBooksWhenEmpty(t *testing.T) {
	t.Parallel()
	svc := &mockSearchService{count: 0}
	catalog := &mockCatalogClient{
		books: []*catalogv1.Book{
			{Id: "1", Title: "Go Book", Author: "Author1"},
			{Id: "2", Title: "Rust Book", Author: "Author2"},
		},
	}

	err := bootstrap.Run(context.Background(), catalog, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svc.upserted) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(svc.upserted))
	}
	if svc.upserted[0].Title != "Go Book" {
		t.Errorf("expected first book 'Go Book', got %s", svc.upserted[0].Title)
	}
}
