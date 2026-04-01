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

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/catalog/internal/handler"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

// noopPublisher is a no-op EventPublisher that discards all events.
type noopPublisher struct{}

func (p *noopPublisher) Publish(_ context.Context, _ service.BookEvent) error { return nil }

// setupPostgres starts a Postgres testcontainer, runs catalog migrations, and
// returns a connected *gorm.DB. The container is terminated via t.Cleanup.
// Duplicated from other test packages because separate packages cannot share
// unexported helpers.
func setupPostgres(t *testing.T) *gorm.DB {
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

// setupKafka starts a Kafka testcontainer and returns its broker addresses.
// The container is terminated via t.Cleanup.
func setupKafka(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-kafka:7.5.0",
		ExposedPorts: []string{"9092/tcp"},
		Env: map[string]string{
			"KAFKA_NODE_ID":                        "1",
			"KAFKA_PROCESS_ROLES":                  "broker,controller",
			"KAFKA_LISTENERS":                      "PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093",
			"KAFKA_ADVERTISED_LISTENERS":            "PLAINTEXT://localhost:9092",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":        "1@localhost:9093",
			"KAFKA_CONTROLLER_LISTENER_NAMES":       "CONTROLLER",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP": "PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"CLUSTER_ID":                           "MkU3OEVBNTcwNTJENDM2Qg",
		},
		WaitingFor: wait.ForLog("Kafka Server started").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start kafka container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate kafka container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get kafka host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9092")
	if err != nil {
		t.Fatalf("failed to get kafka port: %v", err)
	}

	return []string{host + ":" + port.Port()}
}

// startCatalogServer spins up an in-process gRPC server over a bufconn listener,
// registers the CatalogHandler backed by svc with the UnaryAuthInterceptor,
// and returns a connected client. The server and connection are cleaned up via
// t.Cleanup.
func startCatalogServer(t *testing.T, svc *service.CatalogService, jwtSecret string) catalogv1.CatalogServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(pkgauth.UnaryAuthInterceptor(jwtSecret, nil)),
	)
	catalogv1.RegisterCatalogServiceServer(srv, handler.NewCatalogHandler(svc))

	go func() {
		if err := srv.Serve(lis); err != nil {
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
// metadata, suitable for any client call that requires authentication.
func adminCtx(t *testing.T, jwtSecret string) context.Context {
	t.Helper()

	token, err := pkgauth.GenerateToken(uuid.New(), "admin", jwtSecret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// TestCatalog_E2E exercises the full catalog CRUD lifecycle through the gRPC
// interface backed by a real Postgres database (testcontainers). Events are
// discarded via noopPublisher so Kafka is not required for this test.
func TestCatalog_E2E(t *testing.T) {
	const secret = "e2e-secret"

	db := setupPostgres(t)
	repo := repository.NewBookRepository(db)
	svc := service.NewCatalogService(repo, &noopPublisher{})
	client := startCatalogServer(t, svc, secret)
	ctx := adminCtx(t, secret)

	// a. Create a book — verify non-empty ID and title match.
	created, err := client.CreateBook(ctx, &catalogv1.CreateBookRequest{
		Title:         "The Go Programming Language",
		Author:        "Alan Donovan",
		Isbn:          "978-0-13-468599-1",
		Genre:         "Technology",
		Description:   "A definitive reference for Go programmers.",
		PublishedYear: 2015,
		TotalCopies:   3,
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	if created.GetId() == "" {
		t.Fatal("expected non-empty book ID after create")
	}
	if created.GetTitle() != "The Go Programming Language" {
		t.Errorf("create: title mismatch: want %q, got %q", "The Go Programming Language", created.GetTitle())
	}

	bookID := created.GetId()

	// b. Get the book by ID — verify fields match.
	fetched, err := client.GetBook(ctx, &catalogv1.GetBookRequest{Id: bookID})
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if fetched.GetId() != bookID {
		t.Errorf("get: ID mismatch: want %q, got %q", bookID, fetched.GetId())
	}
	if fetched.GetTitle() != created.GetTitle() {
		t.Errorf("get: title mismatch: want %q, got %q", created.GetTitle(), fetched.GetTitle())
	}
	if fetched.GetAuthor() != created.GetAuthor() {
		t.Errorf("get: author mismatch: want %q, got %q", created.GetAuthor(), fetched.GetAuthor())
	}
	if fetched.GetIsbn() != created.GetIsbn() {
		t.Errorf("get: isbn mismatch: want %q, got %q", created.GetIsbn(), fetched.GetIsbn())
	}
	if fetched.GetTotalCopies() != created.GetTotalCopies() {
		t.Errorf("get: total_copies mismatch: want %d, got %d", created.GetTotalCopies(), fetched.GetTotalCopies())
	}

	// c. List books — verify count is 1.
	listResp, err := client.ListBooks(ctx, &catalogv1.ListBooksRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	if listResp.GetTotalCount() != 1 {
		t.Errorf("list: expected total_count 1, got %d", listResp.GetTotalCount())
	}
	if len(listResp.GetBooks()) != 1 {
		t.Errorf("list: expected 1 book in slice, got %d", len(listResp.GetBooks()))
	}

	// d. Update the book title — verify response has new title.
	updated, err := client.UpdateBook(ctx, &catalogv1.UpdateBookRequest{
		Id:            bookID,
		Title:         "The Go Programming Language (2nd Ed.)",
		Author:        created.GetAuthor(),
		Isbn:          created.GetIsbn(),
		Genre:         created.GetGenre(),
		Description:   created.GetDescription(),
		PublishedYear: created.GetPublishedYear(),
		TotalCopies:   created.GetTotalCopies(),
	})
	if err != nil {
		t.Fatalf("UpdateBook: %v", err)
	}
	if updated.GetTitle() != "The Go Programming Language (2nd Ed.)" {
		t.Errorf("update: title mismatch: want %q, got %q", "The Go Programming Language (2nd Ed.)", updated.GetTitle())
	}

	// e. Delete the book — verify success (no error).
	_, err = client.DeleteBook(ctx, &catalogv1.DeleteBookRequest{Id: bookID})
	if err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}

	// f. Get the deleted book — verify codes.NotFound.
	_, err = client.GetBook(ctx, &catalogv1.GetBookRequest{Id: bookID})
	if err == nil {
		t.Fatal("expected error fetching deleted book, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound after delete, got %v", st.Code())
	}
}
