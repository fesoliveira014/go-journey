package handler

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
)

// CatalogHandler implements the generated catalogv1.CatalogServiceServer interface.
type CatalogHandler struct {
	catalogv1.UnimplementedCatalogServiceServer
	svc *service.CatalogService
}

// NewCatalogHandler creates a new gRPC handler backed by the given service.
func NewCatalogHandler(svc *service.CatalogService) *CatalogHandler {
	return &CatalogHandler{svc: svc}
}

func (h *CatalogHandler) CreateBook(ctx context.Context, req *catalogv1.CreateBookRequest) (*catalogv1.Book, error) {
	if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	if req.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}
	if req.GetAuthor() == "" {
		return nil, status.Error(codes.InvalidArgument, "author is required")
	}

	book := &model.Book{
		Title:         req.GetTitle(),
		Author:        req.GetAuthor(),
		ISBN:          req.GetIsbn(),
		Genre:         req.GetGenre(),
		Description:   req.GetDescription(),
		PublishedYear: int(req.GetPublishedYear()),
		TotalCopies:   int(req.GetTotalCopies()),
	}

	created, err := h.svc.CreateBook(ctx, book)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return bookToProto(created), nil
}

func (h *CatalogHandler) GetBook(ctx context.Context, req *catalogv1.GetBookRequest) (*catalogv1.Book, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	book, err := h.svc.GetBook(ctx, id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return bookToProto(book), nil
}

func (h *CatalogHandler) UpdateBook(ctx context.Context, req *catalogv1.UpdateBookRequest) (*catalogv1.Book, error) {
	if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	book := &model.Book{
		ID:            id,
		Title:         req.GetTitle(),
		Author:        req.GetAuthor(),
		ISBN:          req.GetIsbn(),
		Genre:         req.GetGenre(),
		Description:   req.GetDescription(),
		PublishedYear: int(req.GetPublishedYear()),
		TotalCopies:   int(req.GetTotalCopies()),
	}

	updated, err := h.svc.UpdateBook(ctx, book)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return bookToProto(updated), nil
}

func (h *CatalogHandler) DeleteBook(ctx context.Context, req *catalogv1.DeleteBookRequest) (*catalogv1.DeleteBookResponse, error) {
	if err := pkgauth.RequireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	if err := h.svc.DeleteBook(ctx, id); err != nil {
		return nil, toGRPCError(err)
	}
	return &catalogv1.DeleteBookResponse{}, nil
}

func (h *CatalogHandler) ListBooks(ctx context.Context, req *catalogv1.ListBooksRequest) (*catalogv1.ListBooksResponse, error) {
	filter := model.BookFilter{
		Genre:         req.GetGenre(),
		Author:        req.GetAuthor(),
		AvailableOnly: req.GetAvailableOnly(),
	}
	page := model.Pagination{
		Page:     int(req.GetPage()),
		PageSize: int(req.GetPageSize()),
	}

	books, total, err := h.svc.ListBooks(ctx, filter, page)
	if err != nil {
		return nil, toGRPCError(err)
	}

	pbBooks := make([]*catalogv1.Book, len(books))
	for i, b := range books {
		pbBooks[i] = bookToProto(b)
	}
	return &catalogv1.ListBooksResponse{
		Books:      pbBooks,
		TotalCount: int32(total),
	}, nil
}

func (h *CatalogHandler) UpdateAvailability(ctx context.Context, req *catalogv1.UpdateAvailabilityRequest) (*catalogv1.UpdateAvailabilityResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid book ID")
	}

	book, err := h.svc.UpdateAvailability(ctx, id, int(req.GetDelta()))
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &catalogv1.UpdateAvailabilityResponse{
		AvailableCopies: int32(book.AvailableCopies),
	}, nil
}

// bookToProto converts a domain Book to its protobuf representation.
func bookToProto(b *model.Book) *catalogv1.Book {
	return &catalogv1.Book{
		Id:              b.ID.String(),
		Title:           b.Title,
		Author:          b.Author,
		Isbn:            b.ISBN,
		Genre:           b.Genre,
		Description:     b.Description,
		PublishedYear:   int32(b.PublishedYear),
		TotalCopies:     int32(b.TotalCopies),
		AvailableCopies: int32(b.AvailableCopies),
		CreatedAt:       timestamppb.New(b.CreatedAt),
		UpdatedAt:       timestamppb.New(b.UpdatedAt),
	}
}

// toGRPCError translates domain errors to gRPC status errors.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, model.ErrBookNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, model.ErrDuplicateISBN):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, model.ErrInvalidBook):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
