package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
	"github.com/fesoliveira014/library-system/services/reservation/internal/handler"
	"github.com/fesoliveira014/library-system/services/reservation/internal/model"
	"github.com/fesoliveira014/library-system/services/reservation/internal/service"
)

type mockService struct {
	createFn   func(ctx context.Context, bookID uuid.UUID) (*model.Reservation, error)
	returnFn   func(ctx context.Context, resID uuid.UUID) (*model.Reservation, error)
	getFn      func(ctx context.Context, resID uuid.UUID) (*model.Reservation, error)
	listFn     func(ctx context.Context) ([]*model.Reservation, error)
	listAllFn  func(ctx context.Context) ([]service.ReservationDetail, error)
}

func (m *mockService) CreateReservation(ctx context.Context, bookID uuid.UUID) (*model.Reservation, error) {
	return m.createFn(ctx, bookID)
}
func (m *mockService) ReturnBook(ctx context.Context, resID uuid.UUID) (*model.Reservation, error) {
	return m.returnFn(ctx, resID)
}
func (m *mockService) GetReservation(ctx context.Context, resID uuid.UUID) (*model.Reservation, error) {
	return m.getFn(ctx, resID)
}
func (m *mockService) ListUserReservations(ctx context.Context) ([]*model.Reservation, error) {
	return m.listFn(ctx)
}
func (m *mockService) ListAllReservations(ctx context.Context) ([]service.ReservationDetail, error) {
	return m.listAllFn(ctx)
}

func userCtx(userID uuid.UUID) context.Context {
	return pkgauth.ContextWithUser(context.Background(), userID, "user")
}

func TestCreateReservation_InvalidBookID(t *testing.T) {
	t.Parallel()
	h := handler.NewReservationHandler(&mockService{})

	_, err := h.CreateReservation(userCtx(uuid.New()), &reservationv1.CreateReservationRequest{
		BookId: "not-a-uuid",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestCreateReservation_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	bookID := uuid.New()
	resID := uuid.New()
	now := time.Now()

	svc := &mockService{
		createFn: func(_ context.Context, bid uuid.UUID) (*model.Reservation, error) {
			return &model.Reservation{
				ID: resID, UserID: userID, BookID: bid,
				Status: model.StatusActive, ReservedAt: now,
				DueAt: now.Add(14 * 24 * time.Hour),
			}, nil
		},
	}
	h := handler.NewReservationHandler(svc)

	resp, err := h.CreateReservation(userCtx(userID), &reservationv1.CreateReservationRequest{
		BookId: bookID.String(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Reservation.Id != resID.String() {
		t.Errorf("expected reservation ID %s, got %s", resID, resp.Reservation.Id)
	}
}

func TestReturnBook_NotFound(t *testing.T) {
	t.Parallel()
	svc := &mockService{
		returnFn: func(_ context.Context, _ uuid.UUID) (*model.Reservation, error) {
			return nil, model.ErrReservationNotFound
		},
	}
	h := handler.NewReservationHandler(svc)

	_, err := h.ReturnBook(userCtx(uuid.New()), &reservationv1.ReturnBookRequest{
		ReservationId: uuid.New().String(),
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestListUserReservations_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	now := time.Now()

	svc := &mockService{
		listFn: func(_ context.Context) ([]*model.Reservation, error) {
			return []*model.Reservation{
				{ID: uuid.New(), UserID: userID, BookID: uuid.New(),
					Status: model.StatusActive, ReservedAt: now,
					DueAt: now.Add(14 * 24 * time.Hour)},
			}, nil
		},
	}
	h := handler.NewReservationHandler(svc)

	resp, err := h.ListUserReservations(userCtx(userID), &reservationv1.ListUserReservationsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Reservations) != 1 {
		t.Errorf("expected 1 reservation, got %d", len(resp.Reservations))
	}
}

func adminCtx(userID uuid.UUID) context.Context {
	return pkgauth.ContextWithUser(context.Background(), userID, "admin")
}

func TestReservationHandler_ListAllReservations_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	bookID := uuid.New()
	now := time.Now()

	svc := &mockService{
		listAllFn: func(_ context.Context) ([]service.ReservationDetail, error) {
			return []service.ReservationDetail{
				{
					Reservation: model.Reservation{
						ID: uuid.New(), UserID: userID, BookID: bookID,
						Status: model.StatusActive, ReservedAt: now,
						DueAt: now.Add(14 * 24 * time.Hour),
					},
					BookTitle: "Go Programming",
					UserEmail: "admin@example.com",
				},
			}, nil
		},
	}
	h := handler.NewReservationHandler(svc)

	resp, err := h.ListAllReservations(adminCtx(userID), &reservationv1.ListAllReservationsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Reservations) != 1 {
		t.Fatalf("expected 1 reservation detail, got %d", len(resp.Reservations))
	}
	d := resp.Reservations[0]
	if d.BookTitle != "Go Programming" {
		t.Errorf("expected book title %q, got %q", "Go Programming", d.BookTitle)
	}
	if d.UserEmail != "admin@example.com" {
		t.Errorf("expected user email %q, got %q", "admin@example.com", d.UserEmail)
	}
	if d.Status != model.StatusActive {
		t.Errorf("expected status %q, got %q", model.StatusActive, d.Status)
	}
	if d.BookId != bookID.String() {
		t.Errorf("expected book ID %q, got %q", bookID.String(), d.BookId)
	}
}

func TestReservationHandler_ListAllReservations_NonAdmin(t *testing.T) {
	t.Parallel()
	h := handler.NewReservationHandler(&mockService{})

	_, err := h.ListAllReservations(userCtx(uuid.New()), &reservationv1.ListAllReservationsRequest{})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}
