//go:build integration

package handler_test

import (
	"context"
	"fmt"
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

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/catalog/internal/handler"
	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

// mockBookRepo is a minimal in-memory BookRepository used only in the
// bufconn interceptor tests. We use a distinct name to avoid a redeclaration
// conflict with inMemoryRepo in catalog_test.go (same package, different build
// tag — both files are compiled together under -tags integration).
type mockBookRepo struct {
	books map[uuid.UUID]*model.Book
}

func newMockBookRepo() *mockBookRepo {
	return &mockBookRepo{books: make(map[uuid.UUID]*model.Book)}
}

func (r *mockBookRepo) Create(ctx context.Context, book *model.Book) (*model.Book, error) {
	book.ID = uuid.New()
	r.books[book.ID] = book
	return book, nil
}

func (r *mockBookRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error) {
	b, ok := r.books[id]
	if !ok {
		return nil, model.ErrBookNotFound
	}
	return b, nil
}

func (r *mockBookRepo) Update(ctx context.Context, book *model.Book) (*model.Book, error) {
	if _, ok := r.books[book.ID]; !ok {
		return nil, model.ErrBookNotFound
	}
	r.books[book.ID] = book
	return book, nil
}

func (r *mockBookRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := r.books[id]; !ok {
		return model.ErrBookNotFound
	}
	delete(r.books, id)
	return nil
}

func (r *mockBookRepo) List(ctx context.Context, filter model.BookFilter, page model.Pagination) ([]*model.Book, int64, error) {
	var result []*model.Book
	for _, b := range r.books {
		result = append(result, b)
	}
	return result, int64(len(result)), nil
}

func (r *mockBookRepo) UpdateAvailability(ctx context.Context, id uuid.UUID, delta int) error {
	b, ok := r.books[id]
	if !ok {
		return model.ErrBookNotFound
	}
	b.AvailableCopies += delta
	return nil
}

// mockPublisher is a no-op EventPublisher for tests that don't need Kafka.
type mockPublisher struct{}

func (p *mockPublisher) Publish(_ context.Context, _ service.BookEvent) error { return nil }

// startCatalogServer spins up an in-process gRPC server over a bufconn listener,
// registers the CatalogHandler backed by svc, and returns a connected client.
// The server and connection are cleaned up via t.Cleanup.
func startCatalogServer(t *testing.T, svc *service.CatalogService, jwtSecret string) catalogv1.CatalogServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(pkgauth.UnaryAuthInterceptor(jwtSecret, nil)),
	)
	catalogv1.RegisterCatalogServiceServer(srv, handler.NewCatalogHandler(svc))

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Serve returns a non-nil error when GracefulStop is called —
			// that is expected during cleanup.
			_ = err
		}
	}()
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create gRPC client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return catalogv1.NewCatalogServiceClient(conn)
}

// adminCtx returns a context carrying a valid admin JWT in the gRPC outgoing
// metadata, suitable for use with any client call that requires authentication.
func adminCtx(t *testing.T, jwtSecret string) context.Context {
	t.Helper()

	token, err := pkgauth.GenerateToken(uuid.New(), "admin", jwtSecret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// setupPostgresForHandler starts a Postgres testcontainer, runs migrations, and
// returns a connected *gorm.DB. The container is terminated via t.Cleanup.
// This duplicates the helper in the repository integration tests because the
// two packages are separate and cannot share unexported helpers.
func setupPostgresForHandler(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("catalog_test"),
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
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
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

	if err := db.Exec("TRUNCATE TABLE books CASCADE").Error; err != nil {
		t.Fatalf("truncate books: %v", err)
	}

	return db
}

// ---------------------------------------------------------------------------
// Interceptor behaviour tests (mock repo, no Postgres)
// ---------------------------------------------------------------------------

// TestGRPC_CreateBook_Unauthenticated verifies that a request without any
// authorization metadata is rejected with codes.Unauthenticated.
func TestGRPC_CreateBook_Unauthenticated(t *testing.T) {
	svc := service.NewCatalogService(newMockBookRepo(), &mockPublisher{})
	client := startCatalogServer(t, svc, "test-secret")

	// Deliberately use a plain background context — no auth metadata.
	_, err := client.CreateBook(context.Background(), &catalogv1.CreateBookRequest{
		Title:  "Unauthorized Book",
		Author: "Some Author",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", st.Code())
	}
}

// TestGRPC_CreateBook_WithAuth verifies that a properly authenticated admin
// request successfully creates a book.
func TestGRPC_CreateBook_WithAuth(t *testing.T) {
	const secret = "test-secret"
	svc := service.NewCatalogService(newMockBookRepo(), &mockPublisher{})
	client := startCatalogServer(t, svc, secret)

	resp, err := client.CreateBook(adminCtx(t, secret), &catalogv1.CreateBookRequest{
		Title:       "Authenticated Book",
		Author:      "Valid Author",
		TotalCopies: 2,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.GetTitle() != "Authenticated Book" {
		t.Errorf("expected title %q, got %q", "Authenticated Book", resp.GetTitle())
	}
	if resp.GetAvailableCopies() != 2 {
		t.Errorf("expected 2 available copies, got %d", resp.GetAvailableCopies())
	}
}

// TestGRPC_GetBook_NotFound verifies that querying for a non-existent book ID
// returns codes.NotFound over the wire.
func TestGRPC_GetBook_NotFound(t *testing.T) {
	const secret = "test-secret"
	svc := service.NewCatalogService(newMockBookRepo(), &mockPublisher{})
	client := startCatalogServer(t, svc, secret)

	_, err := client.GetBook(adminCtx(t, secret), &catalogv1.GetBookRequest{
		Id: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

// ---------------------------------------------------------------------------
// Full-stack integration tests (bufconn + real Postgres via testcontainers)
// ---------------------------------------------------------------------------

// TestGRPC_Integration_CreateAndGet exercises the complete create-then-get
// path with a real Postgres database.
func TestGRPC_Integration_CreateAndGet(t *testing.T) {
	const secret = "integration-secret"
	db := setupPostgresForHandler(t)
	repo := repository.NewBookRepository(db)
	svc := service.NewCatalogService(repo, &mockPublisher{})
	client := startCatalogServer(t, svc, secret)

	ctx := adminCtx(t, secret)

	created, err := client.CreateBook(ctx, &catalogv1.CreateBookRequest{
		Title:       "Integration Book",
		Author:      "Integration Author",
		Isbn:        "978-0-integration-0",
		TotalCopies: 5,
	})
	if err != nil {
		t.Fatalf("CreateBook failed: %v", err)
	}
	if created.GetId() == "" {
		t.Fatal("expected non-empty book ID")
	}

	fetched, err := client.GetBook(ctx, &catalogv1.GetBookRequest{Id: created.GetId()})
	if err != nil {
		t.Fatalf("GetBook failed: %v", err)
	}
	if fetched.GetTitle() != "Integration Book" {
		t.Errorf("expected title %q, got %q", "Integration Book", fetched.GetTitle())
	}
	if fetched.GetAvailableCopies() != 5 {
		t.Errorf("expected 5 available copies, got %d", fetched.GetAvailableCopies())
	}
}

// TestGRPC_Integration_ListBooks creates several books and verifies that
// ListBooks returns the correct total count.
func TestGRPC_Integration_ListBooks(t *testing.T) {
	const secret = "integration-secret"
	db := setupPostgresForHandler(t)
	repo := repository.NewBookRepository(db)
	svc := service.NewCatalogService(repo, &mockPublisher{})
	client := startCatalogServer(t, svc, secret)

	ctx := adminCtx(t, secret)

	for i := 0; i < 3; i++ {
		_, err := client.CreateBook(ctx, &catalogv1.CreateBookRequest{
			Title:       fmt.Sprintf("List Book %d", i+1),
			Author:      "List Author",
			Isbn:        fmt.Sprintf("978-0-list-%04d", i),
			TotalCopies: 1,
		})
		if err != nil {
			t.Fatalf("CreateBook %d failed: %v", i+1, err)
		}
	}

	resp, err := client.ListBooks(ctx, &catalogv1.ListBooksRequest{
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListBooks failed: %v", err)
	}
	if resp.GetTotalCount() != 3 {
		t.Errorf("expected total count 3, got %d", resp.GetTotalCount())
	}
	if len(resp.GetBooks()) != 3 {
		t.Errorf("expected 3 books in response, got %d", len(resp.GetBooks()))
	}
}
