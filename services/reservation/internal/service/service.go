package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/reservation/internal/model"
)

const loanDuration = 14 * 24 * time.Hour

type ReservationRepository interface {
	Create(ctx context.Context, r *model.Reservation) (*model.Reservation, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Reservation, error)
	CountActive(ctx context.Context, userID uuid.UUID) (int64, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*model.Reservation, error)
	ListAll(ctx context.Context) ([]*model.Reservation, error)
	Update(ctx context.Context, r *model.Reservation) (*model.Reservation, error)
}

type ReservationEvent struct {
	Type          string    `json:"event_type"`
	ReservationID string    `json:"reservation_id"`
	UserID        string    `json:"user_id"`
	BookID        string    `json:"book_id"`
	Timestamp     time.Time `json:"timestamp"`
}

type EventPublisher interface {
	Publish(ctx context.Context, event ReservationEvent) error
}

type ReservationService struct {
	repo      ReservationRepository
	catalog   catalogv1.CatalogServiceClient
	auth      authv1.AuthServiceClient
	publisher EventPublisher
	maxActive int
}

func NewReservationService(
	repo ReservationRepository,
	catalog catalogv1.CatalogServiceClient,
	auth authv1.AuthServiceClient,
	publisher EventPublisher,
	maxActive int,
) *ReservationService {
	return &ReservationService{
		repo:      repo,
		catalog:   catalog,
		auth:      auth,
		publisher: publisher,
		maxActive: maxActive,
	}
}

type ReservationDetail struct {
	model.Reservation
	BookTitle string
	UserEmail string
}

func (s *ReservationService) ListAllReservations(ctx context.Context) ([]ReservationDetail, error) {
	reservations, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	details := make([]ReservationDetail, len(reservations))
	for i, r := range reservations {
		details[i] = ReservationDetail{Reservation: *r}
		book, err := s.catalog.GetBook(ctx, &catalogv1.GetBookRequest{Id: r.BookID.String()})
		if err != nil {
			slog.WarnContext(ctx, "failed to resolve book title", "book_id", r.BookID, "error", err)
			details[i].BookTitle = r.BookID.String()
		} else {
			details[i].BookTitle = book.Title
		}
		user, err := s.auth.GetUser(ctx, &authv1.GetUserRequest{Id: r.UserID.String()})
		if err != nil {
			slog.WarnContext(ctx, "failed to resolve user email", "user_id", r.UserID, "error", err)
			details[i].UserEmail = r.UserID.String()
		} else {
			details[i].UserEmail = user.Email
		}
	}
	return details, nil
}

func (s *ReservationService) CreateReservation(ctx context.Context, bookID uuid.UUID) (*model.Reservation, error) {
	userID, err := pkgauth.UserIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("user not authenticated: %w", err)
	}

	count, err := s.repo.CountActive(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count active reservations: %w", err)
	}
	if count >= int64(s.maxActive) {
		return nil, model.ErrMaxReservations
	}

	book, err := s.catalog.GetBook(ctx, &catalogv1.GetBookRequest{Id: bookID.String()})
	if err != nil {
		return nil, fmt.Errorf("check book availability: %w", err)
	}
	if book.AvailableCopies <= 0 {
		return nil, model.ErrNoAvailableCopies
	}

	now := time.Now()
	res := &model.Reservation{
		UserID:     userID,
		BookID:     bookID,
		Status:     model.StatusActive,
		ReservedAt: now,
		DueAt:      now.Add(loanDuration),
	}
	created, err := s.repo.Create(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("create reservation: %w", err)
	}

	if err := s.publisher.Publish(ctx, ReservationEvent{
		Type:          "reservation.created",
		ReservationID: created.ID.String(),
		UserID:        userID.String(),
		BookID:        bookID.String(),
		Timestamp:     now,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish event", "event", "reservation.created", "reservation_id", created.ID, "error", err)
	}

	return created, nil
}

func (s *ReservationService) ReturnBook(ctx context.Context, reservationID uuid.UUID) (*model.Reservation, error) {
	userID, err := pkgauth.UserIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("user not authenticated: %w", err)
	}

	res, err := s.repo.GetByID(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	if res.UserID != userID {
		return nil, model.ErrPermissionDenied
	}

	if res.Status != model.StatusActive {
		return nil, model.ErrAlreadyReturned
	}

	now := time.Now()
	res.Status = model.StatusReturned
	res.ReturnedAt = &now

	updated, err := s.repo.Update(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("update reservation: %w", err)
	}

	if err := s.publisher.Publish(ctx, ReservationEvent{
		Type:          "reservation.returned",
		ReservationID: updated.ID.String(),
		UserID:        userID.String(),
		BookID:        updated.BookID.String(),
		Timestamp:     now,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish event", "event", "reservation.returned", "reservation_id", updated.ID, "error", err)
	}

	return updated, nil
}

func (s *ReservationService) GetReservation(ctx context.Context, reservationID uuid.UUID) (*model.Reservation, error) {
	userID, err := pkgauth.UserIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("user not authenticated: %w", err)
	}

	res, err := s.repo.GetByID(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	if res.UserID != userID {
		return nil, model.ErrPermissionDenied
	}

	s.expireIfDue(ctx, res)
	return res, nil
}

func (s *ReservationService) ListUserReservations(ctx context.Context) ([]*model.Reservation, error) {
	userID, err := pkgauth.UserIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("user not authenticated: %w", err)
	}

	list, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list reservations: %w", err)
	}

	for _, r := range list {
		s.expireIfDue(ctx, r)
	}

	return list, nil
}

func (s *ReservationService) expireIfDue(ctx context.Context, r *model.Reservation) {
	if r.Status != model.StatusActive || time.Now().Before(r.DueAt) {
		return
	}

	r.Status = model.StatusExpired
	if _, err := s.repo.Update(ctx, r); err != nil {
		slog.ErrorContext(ctx, "failed to expire reservation", "reservation_id", r.ID, "error", err)
		r.Status = model.StatusActive // revert in-memory change
		return
	}

	userID, _ := pkgauth.UserIDFromContext(ctx)
	if err := s.publisher.Publish(ctx, ReservationEvent{
		Type:          "reservation.expired",
		ReservationID: r.ID.String(),
		UserID:        userID.String(),
		BookID:        r.BookID.String(),
		Timestamp:     time.Now(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish event", "event", "reservation.expired", "reservation_id", r.ID, "error", err)
	}
}
