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
	reset    bool
	count    int64
	upserted []model.BookDocument
}

func (m *mockSearchService) EnsureIndex(_ context.Context) error {
	m.ensured = true
	return nil
}

func (m *mockSearchService) ResetIndex(_ context.Context) error {
	m.reset = true
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

	err := bootstrap.Run(context.Background(), catalog, svc, bootstrap.ModeIfEmpty)
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

	err := bootstrap.Run(context.Background(), catalog, svc, bootstrap.ModeIfEmpty)
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

func TestBootstrap_AlwaysResetsAndIndexesEvenWhenPopulated(t *testing.T) {
	t.Parallel()
	svc := &mockSearchService{count: 5}
	catalog := &mockCatalogClient{
		books: []*catalogv1.Book{
			{Id: "1", Title: "Go Book", Author: "Author1"},
		},
	}

	err := bootstrap.Run(context.Background(), catalog, svc, bootstrap.ModeAlways)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.reset {
		t.Fatal("expected ResetIndex to be called")
	}
	if !svc.ensured {
		t.Fatal("expected EnsureIndex to be called")
	}
	if len(svc.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(svc.upserted))
	}
}

func TestBootstrap_DisabledEnsuresIndexOnly(t *testing.T) {
	t.Parallel()
	svc := &mockSearchService{count: 0}
	catalog := &mockCatalogClient{
		books: []*catalogv1.Book{{Id: "1", Title: "Go Book"}},
	}

	err := bootstrap.Run(context.Background(), catalog, svc, bootstrap.ModeDisabled)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.ensured {
		t.Fatal("expected EnsureIndex to be called")
	}
	if svc.reset {
		t.Fatal("did not expect ResetIndex to be called")
	}
	if len(svc.upserted) != 0 {
		t.Fatalf("expected no upserts, got %d", len(svc.upserted))
	}
}

func TestParseMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    bootstrap.Mode
		wantErr bool
	}{
		{name: "default", value: "", want: bootstrap.ModeIfEmpty},
		{name: "if empty", value: "if_empty", want: bootstrap.ModeIfEmpty},
		{name: "always", value: "always", want: bootstrap.ModeAlways},
		{name: "disabled", value: "disabled", want: bootstrap.ModeDisabled},
		{name: "invalid", value: "sometimes", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.ParseMode(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
