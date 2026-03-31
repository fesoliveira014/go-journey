package handler

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

// Service defines the interface the handler depends on.
type Service interface {
	Search(ctx context.Context, query string, filters index.SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error)
	Suggest(ctx context.Context, prefix string, limit int) ([]model.Suggestion, error)
}

// SearchHandler implements the generated searchv1.SearchServiceServer.
type SearchHandler struct {
	searchv1.UnimplementedSearchServiceServer
	svc Service
}

// NewSearchHandler creates a new gRPC handler backed by the given service.
func NewSearchHandler(svc Service) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) Search(ctx context.Context, req *searchv1.SearchRequest) (*searchv1.SearchResponse, error) {
	if req.GetQuery() == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	filters := index.SearchFilters{
		Genre:         req.GetGenre(),
		Author:        req.GetAuthor(),
		AvailableOnly: req.GetAvailableOnly(),
	}

	docs, totalHits, queryTimeMs, err := h.svc.Search(ctx, req.GetQuery(), filters, int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, status.Error(codes.Internal, "search failed")
	}

	books := make([]*searchv1.BookResult, len(docs))
	for i, d := range docs {
		books[i] = &searchv1.BookResult{
			Id:              d.ID,
			Title:           d.Title,
			Author:          d.Author,
			Isbn:            d.ISBN,
			Genre:           d.Genre,
			Description:     d.Description,
			PublishedYear:   int32(d.PublishedYear),
			TotalCopies:     int32(d.TotalCopies),
			AvailableCopies: int32(d.AvailableCopies),
		}
	}

	return &searchv1.SearchResponse{
		Books:       books,
		TotalHits:   totalHits,
		QueryTimeMs: queryTimeMs,
	}, nil
}

func (h *SearchHandler) Suggest(ctx context.Context, req *searchv1.SuggestRequest) (*searchv1.SuggestResponse, error) {
	if req.GetPrefix() == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	suggestions, err := h.svc.Suggest(ctx, req.GetPrefix(), int(req.GetLimit()))
	if err != nil {
		return nil, status.Error(codes.Internal, "suggest failed")
	}

	pbSuggestions := make([]*searchv1.Suggestion, len(suggestions))
	for i, s := range suggestions {
		pbSuggestions[i] = &searchv1.Suggestion{
			BookId: s.BookID,
			Title:  s.Title,
			Author: s.Author,
		}
	}

	return &searchv1.SuggestResponse{Suggestions: pbSuggestions}, nil
}
