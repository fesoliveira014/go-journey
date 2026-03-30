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
)

type mockService struct {
	createFn func(ctx context.Context, bookID uuid.UUID) (*model.Reservation, error)
	returnFn func(ctx context.Context, resID uuid.UUID) (*model.Reservation, error)
	getFn    func(ctx context.Context, resID uuid.UUID) (*model.Reservation, error)
	listFn   func(ctx context.Context) ([]*model.Reservation, error)
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

func userCtx(userID uuid.UUID) context.Context {
	return pkgauth.ContextWithUser(context.Background(), userID, "user")
}

func TestCreateReservation_InvalidBookID(t *testing.T) {
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
