package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/reservation/internal/model"
	"github.com/fesoliveira014/library-system/services/reservation/internal/service"
)

// --- Mocks ---

type mockRepo struct {
	createFn               func(ctx context.Context, r *model.Reservation) (*model.Reservation, error)
	getByIDFn              func(ctx context.Context, id uuid.UUID) (*model.Reservation, error)
	countActiveFn          func(ctx context.Context, userID uuid.UUID) (int64, error)
	listByUserFn           func(ctx context.Context, userID uuid.UUID) ([]*model.Reservation, error)
	listAllFn              func(ctx context.Context) ([]*model.Reservation, error)
	updateFn               func(ctx context.Context, r *model.Reservation) (*model.Reservation, error)
	listDueForExpirationFn func(ctx context.Context, now time.Time) ([]*model.Reservation, error)
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
func (m *mockRepo) ListAll(ctx context.Context) ([]*model.Reservation, error) {
	return m.listAllFn(ctx)
}
func (m *mockRepo) Update(ctx context.Context, r *model.Reservation) (*model.Reservation, error) {
	return m.updateFn(ctx, r)
}
func (m *mockRepo) ListDueForExpiration(ctx context.Context, now time.Time) ([]*model.Reservation, error) {
	return m.listDueForExpirationFn(ctx, now)
}

type mockCatalog struct {
	getBookFn            func(ctx context.Context, in *catalogv1.GetBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error)
	updateAvailabilityFn func(ctx context.Context, in *catalogv1.UpdateAvailabilityRequest, opts ...grpc.CallOption) (*catalogv1.UpdateAvailabilityResponse, error)
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
func (m *mockCatalog) UpdateAvailability(ctx context.Context, in *catalogv1.UpdateAvailabilityRequest, opts ...grpc.CallOption) (*catalogv1.UpdateAvailabilityResponse, error) {
	if m.updateAvailabilityFn == nil {
		return &catalogv1.UpdateAvailabilityResponse{}, nil
	}
	return m.updateAvailabilityFn(ctx, in, opts...)
}

type mockPublisher struct {
	events []service.ReservationEvent
}

func (m *mockPublisher) Publish(_ context.Context, event service.ReservationEvent) error {
	m.events = append(m.events, event)
	return nil
}

type mockAuthClient struct {
	getUserFn func(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.User, error)
}

func (m *mockAuthClient) Register(context.Context, *authv1.RegisterRequest, ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) Login(context.Context, *authv1.LoginRequest, ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) ValidateToken(context.Context, *authv1.ValidateTokenRequest, ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) GetUser(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.User, error) {
	return m.getUserFn(ctx, in, opts...)
}
func (m *mockAuthClient) InitOAuth2(context.Context, *authv1.InitOAuth2Request, ...grpc.CallOption) (*authv1.InitOAuth2Response, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) CompleteOAuth2(context.Context, *authv1.CompleteOAuth2Request, ...grpc.CallOption) (*authv1.AuthResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
func (m *mockAuthClient) ListUsers(context.Context, *authv1.ListUsersRequest, ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// --- Tests ---

func userCtx(userID uuid.UUID) context.Context {
	return pkgauth.ContextWithUser(context.Background(), userID, "user")
}

func TestCreateReservation_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	bookID := uuid.New()
	pub := &mockPublisher{}

	var decrementCalled bool
	repo := &mockRepo{
		countActiveFn: func(_ context.Context, _ uuid.UUID) (int64, error) { return 0, nil },
		createFn: func(_ context.Context, r *model.Reservation) (*model.Reservation, error) {
			r.ID = uuid.New()
			return r, nil
		},
	}
	catalog := &mockCatalog{
		updateAvailabilityFn: func(_ context.Context, in *catalogv1.UpdateAvailabilityRequest, _ ...grpc.CallOption) (*catalogv1.UpdateAvailabilityResponse, error) {
			if in.Delta != -1 {
				t.Errorf("expected decrement of -1, got %d", in.Delta)
			}
			if in.Id != bookID.String() {
				t.Errorf("expected book id %s, got %s", bookID, in.Id)
			}
			decrementCalled = true
			return &catalogv1.UpdateAvailabilityResponse{AvailableCopies: 2}, nil
		},
	}

	svc := service.NewReservationService(repo, catalog, nil, pub, 5)
	res, err := svc.CreateReservation(userCtx(userID), bookID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decrementCalled {
		t.Error("expected UpdateAvailability(-1) to be called")
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
	t.Parallel()
	userID := uuid.New()
	pub := &mockPublisher{}

	repo := &mockRepo{
		countActiveFn: func(_ context.Context, _ uuid.UUID) (int64, error) { return 5, nil },
	}
	catalog := &mockCatalog{}

	svc := service.NewReservationService(repo, catalog, nil, pub, 5)
	_, err := svc.CreateReservation(userCtx(userID), uuid.New())
	if err == nil {
		t.Fatal("expected error for max reservations")
	}
	if err != model.ErrMaxReservations {
		t.Errorf("expected ErrMaxReservations, got %v", err)
	}
}

func TestCreateReservation_NoAvailableCopies(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	pub := &mockPublisher{}

	var createCalled bool
	repo := &mockRepo{
		countActiveFn: func(_ context.Context, _ uuid.UUID) (int64, error) { return 0, nil },
		createFn: func(_ context.Context, r *model.Reservation) (*model.Reservation, error) {
			createCalled = true
			return r, nil
		},
	}
	catalog := &mockCatalog{
		updateAvailabilityFn: func(_ context.Context, _ *catalogv1.UpdateAvailabilityRequest, _ ...grpc.CallOption) (*catalogv1.UpdateAvailabilityResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "no available copies")
		},
	}

	svc := service.NewReservationService(repo, catalog, nil, pub, 5)
	_, err := svc.CreateReservation(userCtx(userID), uuid.New())
	if err != model.ErrNoAvailableCopies {
		t.Errorf("expected ErrNoAvailableCopies, got %v", err)
	}
	if createCalled {
		t.Error("reservation row must not be created when availability decrement fails")
	}
	if len(pub.events) != 0 {
		t.Errorf("expected no events on failed reservation, got %d", len(pub.events))
	}
}

func TestCreateReservation_CompensatesOnCreateFailure(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	bookID := uuid.New()
	pub := &mockPublisher{}

	var deltas []int32
	repo := &mockRepo{
		countActiveFn: func(_ context.Context, _ uuid.UUID) (int64, error) { return 0, nil },
		createFn: func(_ context.Context, _ *model.Reservation) (*model.Reservation, error) {
			return nil, errAssert("db write failed")
		},
	}
	catalog := &mockCatalog{
		updateAvailabilityFn: func(_ context.Context, in *catalogv1.UpdateAvailabilityRequest, _ ...grpc.CallOption) (*catalogv1.UpdateAvailabilityResponse, error) {
			deltas = append(deltas, in.Delta)
			return &catalogv1.UpdateAvailabilityResponse{}, nil
		},
	}

	svc := service.NewReservationService(repo, catalog, nil, pub, 5)
	_, err := svc.CreateReservation(userCtx(userID), bookID)
	if err == nil {
		t.Fatal("expected reservation create to fail")
	}

	// Must decrement then compensate with an increment.
	if len(deltas) != 2 {
		t.Fatalf("expected 2 availability calls (decrement + compensate), got %d: %v", len(deltas), deltas)
	}
	if deltas[0] != -1 {
		t.Errorf("expected first call delta=-1, got %d", deltas[0])
	}
	if deltas[1] != 1 {
		t.Errorf("expected compensating call delta=+1, got %d", deltas[1])
	}
	if len(pub.events) != 0 {
		t.Errorf("expected no events on failed reservation, got %d", len(pub.events))
	}
}

// errAssert is a tiny local error type used by the compensation test. We
// don't care about the value — just that it surfaces from repo.Create.
type errAssert string

func (e errAssert) Error() string { return string(e) }

func TestReturnBook_Success(t *testing.T) {
	t.Parallel()
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

	svc := service.NewReservationService(repo, nil, nil, pub, 5)
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
	t.Parallel()
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

	svc := service.NewReservationService(repo, nil, nil, &mockPublisher{}, 5)
	_, err := svc.ReturnBook(userCtx(otherID), resID)
	if err == nil {
		t.Fatal("expected permission denied error")
	}
}

func TestReturnBook_AlreadyReturned(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	resID := uuid.New()

	repo := &mockRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Reservation, error) {
			return &model.Reservation{
				ID: resID, UserID: userID, Status: model.StatusReturned,
			}, nil
		},
	}

	svc := service.NewReservationService(repo, nil, nil, &mockPublisher{}, 5)
	_, err := svc.ReturnBook(userCtx(userID), resID)
	if err != model.ErrAlreadyReturned {
		t.Errorf("expected ErrAlreadyReturned, got %v", err)
	}
}

func TestListUserReservations_ExpiresOnRead(t *testing.T) {
	t.Parallel()
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

	svc := service.NewReservationService(repo, nil, nil, pub, 5)
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

func TestReapExpired_FlipsOverdueRowsAndPublishes(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	overdue := &model.Reservation{
		ID: uuid.New(), UserID: userID, BookID: uuid.New(),
		Status: model.StatusActive,
		DueAt:  time.Now().Add(-1 * time.Hour),
	}
	pub := &mockPublisher{}
	var listCalledWith time.Time

	repo := &mockRepo{
		listDueForExpirationFn: func(_ context.Context, now time.Time) ([]*model.Reservation, error) {
			listCalledWith = now
			return []*model.Reservation{overdue}, nil
		},
		updateFn: func(_ context.Context, r *model.Reservation) (*model.Reservation, error) {
			return r, nil
		},
	}

	svc := service.NewReservationService(repo, nil, nil, pub, 5)
	svc.ReapExpired(context.Background())

	if listCalledWith.IsZero() {
		t.Error("expected ListDueForExpiration to be called with a non-zero time")
	}
	if overdue.Status != model.StatusExpired {
		t.Errorf("expected status expired, got %s", overdue.Status)
	}
	if len(pub.events) != 1 || pub.events[0].Type != "reservation.expired" {
		t.Errorf("expected one reservation.expired event, got %v", pub.events)
	}
	// The reaper runs without a user-attached context, so the event must
	// fall back to the reservation's own UserID.
	if pub.events[0].UserID != userID.String() {
		t.Errorf("expected event UserID %s, got %s", userID, pub.events[0].UserID)
	}
}

func TestReservationService_ListAllReservations(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	bookID := uuid.New()
	now := time.Now()

	repo := &mockRepo{
		listAllFn: func(_ context.Context) ([]*model.Reservation, error) {
			return []*model.Reservation{
				{
					ID: uuid.New(), UserID: userID, BookID: bookID,
					Status: model.StatusActive, ReservedAt: now,
					DueAt: now.Add(14 * 24 * time.Hour),
				},
			}, nil
		},
	}
	catalog := &mockCatalog{
		getBookFn: func(_ context.Context, in *catalogv1.GetBookRequest, _ ...grpc.CallOption) (*catalogv1.Book, error) {
			return &catalogv1.Book{Id: in.Id, Title: "Test Book"}, nil
		},
	}
	auth := &mockAuthClient{
		getUserFn: func(_ context.Context, in *authv1.GetUserRequest, _ ...grpc.CallOption) (*authv1.User, error) {
			return &authv1.User{Id: in.Id, Email: "user@example.com"}, nil
		},
	}

	svc := service.NewReservationService(repo, catalog, auth, &mockPublisher{}, 5)
	details, err := svc.ListAllReservations(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}
	if details[0].BookTitle != "Test Book" {
		t.Errorf("expected book title %q, got %q", "Test Book", details[0].BookTitle)
	}
	if details[0].UserEmail != "user@example.com" {
		t.Errorf("expected user email %q, got %q", "user@example.com", details[0].UserEmail)
	}
}
