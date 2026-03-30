package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/reservation/internal/model"
	"github.com/fesoliveira014/library-system/services/reservation/internal/service"
)

// --- Mocks ---

type mockRepo struct {
	createFn      func(ctx context.Context, r *model.Reservation) (*model.Reservation, error)
	getByIDFn     func(ctx context.Context, id uuid.UUID) (*model.Reservation, error)
	countActiveFn func(ctx context.Context, userID uuid.UUID) (int64, error)
	listByUserFn  func(ctx context.Context, userID uuid.UUID) ([]*model.Reservation, error)
	updateFn      func(ctx context.Context, r *model.Reservation) (*model.Reservation, error)
}

func (m *mockRepo) Create(ctx context.Context, r *model.Reservation) (*model.Reservation, error) {
	return m.createFn(ctx, r)
}
func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Reservation, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockRepo) CountActive(ctx context.Context, userID uuid.UUID) (int64, error) {
	return m.countActiveFn(ctx, userID)
}
func (m *mockRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]*model.Reservation, error) {
	return m.listByUserFn(ctx, userID)
}
func (m *mockRepo) Update(ctx context.Context, r *model.Reservation) (*model.Reservation, error) {
	return m.updateFn(ctx, r)
}

type mockCatalog struct {
	getBookFn func(ctx context.Context, in *catalogv1.GetBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error)
}

func (m *mockCatalog) ListBooks(context.Context, *catalogv1.ListBooksRequest, ...grpc.CallOption) (*catalogv1.ListBooksResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockCatalog) GetBook(ctx context.Context, in *catalogv1.GetBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error) {
	return m.getBookFn(ctx, in, opts...)
}
func (m *mockCatalog) CreateBook(context.Context, *catalogv1.CreateBookRequest, ...grpc.CallOption) (*catalogv1.Book, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockCatalog) UpdateBook(context.Context, *catalogv1.UpdateBookRequest, ...grpc.CallOption) (*catalogv1.Book, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockCatalog) DeleteBook(context.Context, *catalogv1.DeleteBookRequest, ...grpc.CallOption) (*catalogv1.DeleteBookResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockCatalog) UpdateAvailability(context.Context, *catalogv1.UpdateAvailabilityRequest, ...grpc.CallOption) (*catalogv1.UpdateAvailabilityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

type mockPublisher struct {
	events []service.ReservationEvent
}

func (m *mockPublisher) Publish(_ context.Context, event service.ReservationEvent) error {
	m.events = append(m.events, event)
	return nil
}

// --- Tests ---

func userCtx(userID uuid.UUID) context.Context {
	return pkgauth.ContextWithUser(context.Background(), userID, "user")
}

func TestCreateReservation_Success(t *testing.T) {
	userID := uuid.New()
	bookID := uuid.New()
	pub := &mockPublisher{}

	repo := &mockRepo{
		countActiveFn: func(_ context.Context, _ uuid.UUID) (int64, error) { return 0, nil },
		createFn: func(_ context.Context, r *model.Reservation) (*model.Reservation, error) {
			r.ID = uuid.New()
			return r, nil
		},
	}
	catalog := &mockCatalog{
		getBookFn: func(_ context.Context, in *catalogv1.GetBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
			return &catalogv1.Book{Id: in.Id, AvailableCopies: 3}, nil
		},
	}

	svc := service.NewReservationService(repo, catalog, pub, 5)
	res, err := svc.CreateReservation(userCtx(userID), bookID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != model.StatusActive {
		t.Errorf("expected status active, got %s", res.Status)
	}
	if res.UserID != userID {
		t.Errorf("expected user %s, got %s", userID, res.UserID)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	if pub.events[0].Type != "reservation.created" {
		t.Errorf("expected event type reservation.created, got %s", pub.events[0].Type)
	}
}

func TestCreateReservation_MaxReservations(t *testing.T) {
	userID := uuid.New()
	pub := &mockPublisher{}

	repo := &mockRepo{
		countActiveFn: func(_ context.Context, _ uuid.UUID) (int64, error) { return 5, nil },
	}
	catalog := &mockCatalog{}

	svc := service.NewReservationService(repo, catalog, pub, 5)
	_, err := svc.CreateReservation(userCtx(userID), uuid.New())
	if err == nil {
		t.Fatal("expected error for max reservations")
	}
	if err != model.ErrMaxReservations {
		t.Errorf("expected ErrMaxReservations, got %v", err)
	}
}

func TestCreateReservation_NoAvailableCopies(t *testing.T) {
	userID := uuid.New()
	pub := &mockPublisher{}

	repo := &mockRepo{
		countActiveFn: func(_ context.Context, _ uuid.UUID) (int64, error) { return 0, nil },
	}
	catalog := &mockCatalog{
		getBookFn: func(_ context.Context, _ *catalogv1.GetBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
			return &catalogv1.Book{AvailableCopies: 0}, nil
		},
	}

	svc := service.NewReservationService(repo, catalog, pub, 5)
	_, err := svc.CreateReservation(userCtx(userID), uuid.New())
	if err != model.ErrNoAvailableCopies {
		t.Errorf("expected ErrNoAvailableCopies, got %v", err)
	}
}

func TestReturnBook_Success(t *testing.T) {
	userID := uuid.New()
	resID := uuid.New()
	pub := &mockPublisher{}

	stored := &model.Reservation{
		ID: resID, UserID: userID, BookID: uuid.New(),
		Status: model.StatusActive, ReservedAt: time.Now(),
		DueAt: time.Now().Add(14 * 24 * time.Hour),
	}
	repo := &mockRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Reservation, error) {
			if id == resID {
				return stored, nil
			}
			return nil, model.ErrReservationNotFound
		},
		updateFn: func(_ context.Context, r *model.Reservation) (*model.Reservation, error) {
			return r, nil
		},
	}

	svc := service.NewReservationService(repo, nil, pub, 5)
	res, err := svc.ReturnBook(userCtx(userID), resID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != model.StatusReturned {
		t.Errorf("expected status returned, got %s", res.Status)
	}
	if res.ReturnedAt == nil {
		t.Error("expected returned_at to be set")
	}
	if len(pub.events) != 1 || pub.events[0].Type != "reservation.returned" {
		t.Errorf("expected reservation.returned event, got %v", pub.events)
	}
}

func TestReturnBook_WrongUser(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()
	resID := uuid.New()

	repo := &mockRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Reservation, error) {
			return &model.Reservation{
				ID: resID, UserID: ownerID, Status: model.StatusActive,
			}, nil
		},
	}

	svc := service.NewReservationService(repo, nil, &mockPublisher{}, 5)
	_, err := svc.ReturnBook(userCtx(otherID), resID)
	if err == nil {
		t.Fatal("expected permission denied error")
	}
}

func TestReturnBook_AlreadyReturned(t *testing.T) {
	userID := uuid.New()
	resID := uuid.New()

	repo := &mockRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Reservation, error) {
			return &model.Reservation{
				ID: resID, UserID: userID, Status: model.StatusReturned,
			}, nil
		},
	}

	svc := service.NewReservationService(repo, nil, &mockPublisher{}, 5)
	_, err := svc.ReturnBook(userCtx(userID), resID)
	if err != model.ErrAlreadyReturned {
		t.Errorf("expected ErrAlreadyReturned, got %v", err)
	}
}

func TestListUserReservations_ExpiresOnRead(t *testing.T) {
	userID := uuid.New()
	pub := &mockPublisher{}

	expired := &model.Reservation{
		ID: uuid.New(), UserID: userID, BookID: uuid.New(),
		Status: model.StatusActive,
		DueAt:  time.Now().Add(-1 * time.Hour), // past due
	}
	active := &model.Reservation{
		ID: uuid.New(), UserID: userID, BookID: uuid.New(),
		Status: model.StatusActive,
		DueAt:  time.Now().Add(24 * time.Hour), // not due
	}

	repo := &mockRepo{
		listByUserFn: func(_ context.Context, _ uuid.UUID) ([]*model.Reservation, error) {
			return []*model.Reservation{expired, active}, nil
		},
		updateFn: func(_ context.Context, r *model.Reservation) (*model.Reservation, error) {
			return r, nil
		},
	}

	svc := service.NewReservationService(repo, nil, pub, 5)
	list, err := svc.ListUserReservations(userCtx(userID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
	if expired.Status != model.StatusExpired {
		t.Errorf("expected expired status, got %s", expired.Status)
	}
	if active.Status != model.StatusActive {
		t.Errorf("expected active status, got %s", active.Status)
	}
	if len(pub.events) != 1 || pub.events[0].Type != "reservation.expired" {
		t.Errorf("expected one reservation.expired event, got %v", pub.events)
	}
}
