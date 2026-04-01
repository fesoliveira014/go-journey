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
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/auth/internal/handler"
	"github.com/fesoliveira014/library-system/services/auth/internal/repository"
	"github.com/fesoliveira014/library-system/services/auth/internal/service"
	"github.com/fesoliveira014/library-system/services/auth/migrations"
)

func setupPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("auth_test"),
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
			t.Logf("failed to terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
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

	return db
}

func startAuthServer(t *testing.T, svc *service.AuthService, jwtSecret string) authv1.AuthServiceClient {
	t.Helper()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	skipMethods := []string{
		"/auth.v1.AuthService/Register",
		"/auth.v1.AuthService/Login",
	}

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(pkgauth.UnaryAuthInterceptor(jwtSecret, skipMethods)),
	)

	h := handler.NewAuthHandler(svc)
	authv1.RegisterAuthServiceServer(srv, h)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// server stopped — normal during test cleanup
		}
	}()

	t.Cleanup(func() {
		srv.GracefulStop()
		lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
	})

	return authv1.NewAuthServiceClient(conn)
}

func TestAuth_E2E(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewUserRepository(db)
	svc := service.NewAuthService(repo, "test-secret", "1h")
	client := startAuthServer(t, svc, "test-secret")

	ctx := context.Background()

	// a. Register user — verify non-empty token and matching email.
	regResp, err := client.Register(ctx, &authv1.RegisterRequest{
		Email:    "e2e@example.com",
		Password: "password123",
		Name:     "E2E User",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if regResp.GetToken() == "" {
		t.Error("Register: expected non-empty token")
	}
	if regResp.GetUser().GetEmail() != "e2e@example.com" {
		t.Errorf("Register: expected email %q, got %q", "e2e@example.com", regResp.GetUser().GetEmail())
	}
	registeredUserID := regResp.GetUser().GetId()

	// b. Login with correct credentials — verify token returned.
	loginResp, err := client.Login(ctx, &authv1.LoginRequest{
		Email:    "e2e@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if loginResp.GetToken() == "" {
		t.Error("Login: expected non-empty token")
	}
	loginToken := loginResp.GetToken()

	// c. ValidateToken with the login token — verify user ID matches registered user.
	authCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+loginToken))
	valResp, err := client.ValidateToken(authCtx, &authv1.ValidateTokenRequest{
		Token: loginToken,
	})
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if valResp.GetUserId() != registeredUserID {
		t.Errorf("ValidateToken: expected user ID %q, got %q", registeredUserID, valResp.GetUserId())
	}

	// d. Register same email again — verify AlreadyExists.
	_, err = client.Register(ctx, &authv1.RegisterRequest{
		Email:    "e2e@example.com",
		Password: "anotherpassword",
		Name:     "Duplicate User",
	})
	if err == nil {
		t.Fatal("Register duplicate: expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Register duplicate: expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.AlreadyExists {
		t.Errorf("Register duplicate: expected AlreadyExists, got %v", st.Code())
	}

	// e. Login with wrong password — verify Unauthenticated.
	_, err = client.Login(ctx, &authv1.LoginRequest{
		Email:    "e2e@example.com",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Fatal("Login wrong password: expected error, got nil")
	}
	st, ok = status.FromError(err)
	if !ok {
		t.Fatalf("Login wrong password: expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("Login wrong password: expected Unauthenticated, got %v", st.Code())
	}
}
