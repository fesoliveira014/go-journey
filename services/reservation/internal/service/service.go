package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	ListDueForExpiration(ctx context.Context, now time.Time) ([]*model.Reservation, error)
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

// CreateReservation reserves a copy of bookID for the caller. The flow is
// decrement-then-reserve: we ask catalog to atomically drop available_copies
// by 1, and only if that succeeds do we create the reservation row. This is
// what closes the TOCTOU gap that an older "check availability, then create"
// flow had — two concurrent requests for the last copy would both have seen
// available_copies > 0 and both would have succeeded. With the database as
// the gate (via the guarded UPDATE in catalog), exactly one wins.
//
// If the reservation row fails to persist after the decrement, we compensate
// with an increment so catalog's counter does not drift. The compensation is
// best-effort and logged — the expiration reaper (see RunExpirationReaper)
// provides the backstop for any drift that escapes here.
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

	// Decrement availability first. Catalog returns FailedPrecondition when
	// the book exists but has no copies left; anything else is an infra
	// error we surface as-is.
	if _, err := s.catalog.UpdateAvailability(ctx, &catalogv1.UpdateAvailabilityRequest{
		Id:    bookID.String(),
		Delta: -1,
	}); err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.FailedPrecondition:
				return nil, model.ErrNoAvailableCopies
			case codes.NotFound:
				return nil, fmt.Errorf("book not found: %w", err)
			}
		}
		return nil, fmt.Errorf("reserve availability: %w", err)
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
		// Compensate: the catalog counter was already decremented, so we
		// must put the copy back or the book becomes permanently less
		// available than it really is.
		if _, rollbackErr := s.catalog.UpdateAvailability(ctx, &catalogv1.UpdateAvailabilityRequest{
			Id:    bookID.String(),
			Delta: 1,
		}); rollbackErr != nil {
			slog.ErrorContext(ctx, "failed to compensate availability after reservation create failure",
				"book_id", bookID, "create_error", err, "rollback_error", rollbackErr)
		}
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

// expireIfDue is the read-path trigger: whenever a user reads their own
// reservations, any overdue ones flip to 'expired' as a side effect. It
// intentionally does not guarantee timeliness — a reservation nobody reads
// for a week stays 'active' until the reaper (RunExpirationReaper) sweeps it.
func (s *ReservationService) expireIfDue(ctx context.Context, r *model.Reservation) {
	if r.Status != model.StatusActive || time.Now().Before(r.DueAt) {
		return
	}
	s.expireReservation(ctx, r)
}

// expireReservation flips the row to 'expired', persists it, and publishes
// the event. Caller must have already decided expiration is appropriate.
// On DB failure the in-memory status is reverted so the caller does not
// show stale data to the user.
func (s *ReservationService) expireReservation(ctx context.Context, r *model.Reservation) {
	previousStatus := r.Status
	r.Status = model.StatusExpired
	if _, err := s.repo.Update(ctx, r); err != nil {
		slog.ErrorContext(ctx, "failed to expire reservation", "reservation_id", r.ID, "error", err)
		r.Status = previousStatus // revert in-memory change
		return
	}

	// The reaper runs with a background context that has no user attached,
	// so UserIDFromContext may fail — fall back to the reservation's own
	// UserID so consumers downstream (e.g. catalog) can still route the
	// availability increment correctly.
	var userIDStr string
	if uid, err := pkgauth.UserIDFromContext(ctx); err == nil {
		userIDStr = uid.String()
	} else {
		userIDStr = r.UserID.String()
	}

	if err := s.publisher.Publish(ctx, ReservationEvent{
		Type:          "reservation.expired",
		ReservationID: r.ID.String(),
		UserID:        userIDStr,
		BookID:        r.BookID.String(),
		Timestamp:     time.Now(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish event", "event", "reservation.expired", "reservation_id", r.ID, "error", err)
	}
}

// RunExpirationReaper periodically scans for active reservations whose
// due_at has passed and flips them to 'expired'. This closes the gap left
// by expire-on-read: without it, a book remains unavailable forever if the
// holder never logs in again. Blocks until ctx is cancelled; intended to
// be started in its own goroutine during service startup.
//
// Each tick is a full sweep (one query, N updates). For a library-sized
// workload this is fine; if the active set ever grows into the millions,
// this would want batching and a cursor-style index.
func (s *ReservationService) RunExpirationReaper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	slog.InfoContext(ctx, "reservation expiration reaper started", "interval", interval)
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "reservation expiration reaper stopping")
			return
		case <-ticker.C:
			s.ReapExpired(ctx)
		}
	}
}

// ReapExpired runs a single pass of the expiration sweep. Exported so it
// can be invoked deterministically from tests without waiting for the
// ticker.
func (s *ReservationService) ReapExpired(ctx context.Context) {
	due, err := s.repo.ListDueForExpiration(ctx, time.Now())
	if err != nil {
		slog.ErrorContext(ctx, "reaper: list due reservations failed", "error", err)
		return
	}
	for _, r := range due {
		s.expireReservation(ctx, r)
	}
}
