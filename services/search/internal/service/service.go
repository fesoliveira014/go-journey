package service

import (
	"context"

	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// SearchService provides search and indexing operations.
type SearchService struct {
	index index.IndexRepository
}

// NewSearchService creates a new search service.
func NewSearchService(idx index.IndexRepository) *SearchService {
	return &SearchService{index: idx}
}

func (s *SearchService) Search(ctx context.Context, query string, filters index.SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return s.index.Search(ctx, query, filters, page, pageSize)
}

func (s *SearchService) Suggest(ctx context.Context, prefix string, limit int) ([]model.Suggestion, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	docs, err := s.index.Suggest(ctx, prefix, limit)
	if err != nil {
		return nil, err
	}

	suggestions := make([]model.Suggestion, len(docs))
	for i, d := range docs {
		suggestions[i] = model.Suggestion{
			BookID: d.ID,
			Title:  d.Title,
			Author: d.Author,
		}
	}
	return suggestions, nil
}

func (s *SearchService) Upsert(ctx context.Context, doc model.BookDocument) error {
	return s.index.Upsert(ctx, doc)
}

func (s *SearchService) Delete(ctx context.Context, id string) error {
	return s.index.Delete(ctx, id)
}

func (s *SearchService) EnsureIndex(ctx context.Context) error {
	return s.index.EnsureIndex(ctx)
}

func (s *SearchService) Count(ctx context.Context) (int64, error) {
	return s.index.Count(ctx)
}
