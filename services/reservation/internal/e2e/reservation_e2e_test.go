//go:build integration

package e2e_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/reservation/internal/handler"
	"github.com/fesoliveira014/library-system/services/reservation/internal/repository"
	rsvc "github.com/fesoliveira014/library-system/services/reservation/internal/service"
	"github.com/fesoliveira014/library-system/services/reservation/migrations"
)

// mockCatalogClient satisfies catalogv1.CatalogServiceClient, always returning
// a book with 10 available copies.
type mockCatalogClient struct {
	catalogv1.CatalogServiceClient
}

func (m *mockCatalogClient) GetBook(ctx context.Context, req *catalogv1.GetBookRequest, opts ...grpc.CallOption) (*catalogv1.Book, error) {
	return &catalogv1.Book{
		Id:              req.GetId(),
		AvailableCopies: 10,
	}, nil
}

// mockAuthClient satisfies authv1.AuthServiceClient, always returning
// a user with a test email.
type mockAuthClient struct {
	authv1.AuthServiceClient
}

func (m *mockAuthClient) GetUser(_ context.Context, req *authv1.GetUserRequest, _ ...grpc.CallOption) (*authv1.User, error) {
	return &authv1.User{
		Id:    req.GetId(),
		Email: "test@example.com",
	}, nil
}

// noopPublisher satisfies rsvc.EventPublisher without doing anything.
type noopPublisher struct{}

func (n *noopPublisher) Publish(_ context.Context, _ rsvc.ReservationEvent) error { return nil }

// setupPostgres starts a Postgres testcontainer, runs migrations, and returns a
// *gorm.DB connected to it. The container is terminated on test cleanup.
func setupPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("reservation_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get postgres connection string: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}

	// uuid-ossp is required for uuid_generate_v4() used in the migration.
	if _, err := sqlDB.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`); err != nil {
		t.Fatalf("failed to create uuid-ossp extension: %v", err)
	}

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		t.Fatalf("failed to create migration driver: %v", err)
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("failed to create migration source: %v", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		t.Fatalf("failed to create migrator: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

// setupKafka starts a Kafka testcontainer. It is not used directly here (the
// noopPublisher is used instead), but is kept as a helper for future tests that
// need a real broker.
func setupKafka(t *testing.T) {
	t.Helper()
	// Kafka is not wired into this test; the noopPublisher handles events.
	// A real Kafka container can be started here when needed:
	//   tcKafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
}

// startReservationServer wires up a bufconn gRPC server with the JWT interceptor
// and returns a client connected to it.
func startReservationServer(t *testing.T, svc handler.Service, jwtSecret string) reservationv1.ReservationServiceClient {
	t.Helper()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(pkgauth.UnaryAuthInterceptor(jwtSecret, nil)),
	)

	h := handler.NewReservationHandler(svc)
	reservationv1.RegisterReservationServiceServer(srv, h)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// server stopped — normal during test cleanup
		}
	}()

	t.Cleanup(func() {
		srv.GracefulStop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create gRPC client: %v", err)
	}

	t.Cleanup(func() {
		_ = conn.Close()
	})

	return reservationv1.NewReservationServiceClient(conn)
}

// userCtx generates a signed JWT for userID and returns a context carrying the
// "authorization: Bearer <token>" outgoing metadata.
func userCtx(t *testing.T, userID uuid.UUID, jwtSecret string) context.Context {
	t.Helper()

	token, err := pkgauth.GenerateToken(userID, "user", jwtSecret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewOutgoingContext(context.Background(), md)
}

func TestReservation_E2E(t *testing.T) {
	const (
		jwtSecret = "e2e-test-secret"
		maxActive = 3
	)

	db := setupPostgres(t)
	setupKafka(t) // no-op; retains the helper call for documentation purposes

	repo := repository.NewReservationRepository(db)
	catalogClient := &mockCatalogClient{}
	publisher := &noopPublisher{}
	authClient := &mockAuthClient{}
	svc := rsvc.NewReservationService(repo, catalogClient, authClient, publisher, maxActive)

	client := startReservationServer(t, svc, jwtSecret)

	userID := uuid.New()
	bookID := uuid.New()
	ctx := userCtx(t, userID, jwtSecret)

	// a. Create reservation — verify non-empty ID and status "active".
	createResp, err := client.CreateReservation(ctx, &reservationv1.CreateReservationRequest{
		BookId: bookID.String(),
	})
	if err != nil {
		t.Fatalf("CreateReservation failed: %v", err)
	}
	res := createResp.GetReservation()
	if res.GetId() == "" {
		t.Error("CreateReservation: expected non-empty reservation ID")
	}
	if res.GetStatus() != "active" {
		t.Errorf("CreateReservation: expected status %q, got %q", "active", res.GetStatus())
	}
	reservationID := res.GetId()

	// b. Get reservation — verify fields match the created reservation.
	getResp, err := client.GetReservation(ctx, &reservationv1.GetReservationRequest{
		ReservationId: reservationID,
	})
	if err != nil {
		t.Fatalf("GetReservation failed: %v", err)
	}
	if getResp.GetId() != reservationID {
		t.Errorf("GetReservation: expected ID %q, got %q", reservationID, getResp.GetId())
	}
	if getResp.GetUserId() != userID.String() {
		t.Errorf("GetReservation: expected user ID %q, got %q", userID.String(), getResp.GetUserId())
	}
	if getResp.GetBookId() != bookID.String() {
		t.Errorf("GetReservation: expected book ID %q, got %q", bookID.String(), getResp.GetBookId())
	}
	if getResp.GetStatus() != "active" {
		t.Errorf("GetReservation: expected status %q, got %q", "active", getResp.GetStatus())
	}

	// c. List user reservations — verify count is 1.
	listResp, err := client.ListUserReservations(ctx, &reservationv1.ListUserReservationsRequest{})
	if err != nil {
		t.Fatalf("ListUserReservations failed: %v", err)
	}
	if len(listResp.GetReservations()) != 1 {
		t.Errorf("ListUserReservations: expected 1 reservation, got %d", len(listResp.GetReservations()))
	}

	// d. Return book — verify status becomes "returned".
	returnResp, err := client.ReturnBook(ctx, &reservationv1.ReturnBookRequest{
		ReservationId: reservationID,
	})
	if err != nil {
		t.Fatalf("ReturnBook failed: %v", err)
	}
	if returnResp.GetReservation().GetStatus() != "returned" {
		t.Errorf("ReturnBook: expected status %q, got %q", "returned", returnResp.GetReservation().GetStatus())
	}

	// e. Test max reservations: create maxActive reservations, then attempt one
	//    more and expect codes.ResourceExhausted.
	//    Use a fresh user so the previous reservation does not interfere.
	freshUserID := uuid.New()
	freshCtx := userCtx(t, freshUserID, jwtSecret)

	for i := 0; i < maxActive; i++ {
		_, err := client.CreateReservation(freshCtx, &reservationv1.CreateReservationRequest{
			BookId: uuid.New().String(),
		})
		if err != nil {
			t.Fatalf("max-reservations setup: CreateReservation[%d] failed: %v", i, err)
		}
	}

	_, err = client.CreateReservation(freshCtx, &reservationv1.CreateReservationRequest{
		BookId: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("max-reservations: expected error when exceeding max active reservations, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("max-reservations: expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.ResourceExhausted {
		t.Errorf("max-reservations: expected ResourceExhausted, got %v", st.Code())
	}
}
