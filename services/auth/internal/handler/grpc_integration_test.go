//go:build integration

package handler_test

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

func setupPostgresGRPC(t *testing.T) *gorm.DB {
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

	db.Exec("TRUNCATE TABLE users CASCADE")

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

func TestGRPC_Register(t *testing.T) {
	db := setupPostgresGRPC(t)
	repo := repository.NewUserRepository(db)
	svc := service.NewAuthService(repo, "test-secret", "1h")
	client := startAuthServer(t, svc, "test-secret")

	resp, err := client.Register(context.Background(), &authv1.RegisterRequest{
		Email:    "register@example.com",
		Password: "password123",
		Name:     "Test User",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if resp.GetToken() == "" {
		t.Error("expected non-empty token")
	}
	if resp.GetUser().GetEmail() != "register@example.com" {
		t.Errorf("expected email %q, got %q", "register@example.com", resp.GetUser().GetEmail())
	}
}

func TestGRPC_Login(t *testing.T) {
	db := setupPostgresGRPC(t)
	repo := repository.NewUserRepository(db)
	svc := service.NewAuthService(repo, "test-secret", "1h")
	client := startAuthServer(t, svc, "test-secret")

	_, err := client.Register(context.Background(), &authv1.RegisterRequest{
		Email:    "login@example.com",
		Password: "password123",
		Name:     "Login User",
	})
	if err != nil {
		t.Fatalf("setup: Register failed: %v", err)
	}

	resp, err := client.Login(context.Background(), &authv1.LoginRequest{
		Email:    "login@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.GetToken() == "" {
		t.Error("expected non-empty token")
	}
}

func TestGRPC_Register_DuplicateEmail(t *testing.T) {
	db := setupPostgresGRPC(t)
	repo := repository.NewUserRepository(db)
	svc := service.NewAuthService(repo, "test-secret", "1h")
	client := startAuthServer(t, svc, "test-secret")

	req := &authv1.RegisterRequest{
		Email:    "dup@example.com",
		Password: "password123",
		Name:     "Dup User",
	}

	if _, err := client.Register(context.Background(), req); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	_, err := client.Register(context.Background(), req)
	if err == nil {
		t.Fatal("expected error on duplicate register, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.AlreadyExists {
		t.Errorf("expected AlreadyExists, got %v", st.Code())
	}
}

func TestGRPC_Login_WrongPassword(t *testing.T) {
	db := setupPostgresGRPC(t)
	repo := repository.NewUserRepository(db)
	svc := service.NewAuthService(repo, "test-secret", "1h")
	client := startAuthServer(t, svc, "test-secret")

	if _, err := client.Register(context.Background(), &authv1.RegisterRequest{
		Email:    "wrongpass@example.com",
		Password: "correctpassword",
		Name:     "Wrong Pass User",
	}); err != nil {
		t.Fatalf("setup: Register failed: %v", err)
	}

	_, err := client.Login(context.Background(), &authv1.LoginRequest{
		Email:    "wrongpass@example.com",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Fatal("expected error on wrong password, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestGRPC_ValidateToken(t *testing.T) {
	db := setupPostgresGRPC(t)
	repo := repository.NewUserRepository(db)
	svc := service.NewAuthService(repo, "test-secret", "1h")
	client := startAuthServer(t, svc, "test-secret")

	regResp, err := client.Register(context.Background(), &authv1.RegisterRequest{
		Email:    "validate@example.com",
		Password: "password123",
		Name:     "Validate User",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	token := regResp.GetToken()
	userID := regResp.GetUser().GetId()

	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer "+token),
	)

	valResp, err := client.ValidateToken(ctx, &authv1.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if valResp.GetUserId() != userID {
		t.Errorf("expected user ID %q, got %q", userID, valResp.GetUserId())
	}
}
