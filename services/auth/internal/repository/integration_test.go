//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/auth/internal/model"
	"github.com/fesoliveira014/library-system/services/auth/internal/repository"
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

	db.Exec("TRUNCATE TABLE users CASCADE")

	return db
}

func TestIntegration_CreateUser(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{
		Email:        "create@example.com",
		PasswordHash: strPtr("$2a$10$hashedpassword"),
		Name:         "Create User",
		Role:         "user",
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
}

func TestIntegration_DuplicateEmail(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user1 := &model.User{Email: "dup@example.com", PasswordHash: strPtr("hash"), Name: "User 1", Role: "user"}
	if _, err := repo.Create(ctx, user1); err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	user2 := &model.User{Email: "dup@example.com", PasswordHash: strPtr("hash"), Name: "User 2", Role: "user"}
	_, err := repo.Create(ctx, user2)
	if err != model.ErrDuplicateEmail {
		t.Errorf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestIntegration_GetByEmail(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{Email: "email@example.com", PasswordHash: strPtr("hash"), Name: "Email User", Role: "user"}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("setup: failed to create user: %v", err)
	}

	found, err := repo.GetByEmail(ctx, "email@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected ID %v, got %v", created.ID, found.ID)
	}
	if found.Email != created.Email {
		t.Errorf("expected email %q, got %q", created.Email, found.Email)
	}
}

func TestIntegration_GetByID(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{Email: "getbyid@example.com", PasswordHash: strPtr("hash"), Name: "ID User", Role: "user"}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("setup: failed to create user: %v", err)
	}

	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected ID %v, got %v", created.ID, found.ID)
	}
}

func TestIntegration_GetByID_NotFound(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if err != model.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestIntegration_GetByOAuthID(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{
		Email:         "oauth@example.com",
		Name:          "OAuth User",
		Role:          "user",
		OAuthProvider: strPtr("google"),
		OAuthID:       strPtr("oauth-123"),
	}
	if _, err := repo.Create(ctx, user); err != nil {
		t.Fatalf("setup: failed to create user: %v", err)
	}

	found, err := repo.GetByOAuthID(ctx, "google", "oauth-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Email != "oauth@example.com" {
		t.Errorf("expected email %q, got %q", "oauth@example.com", found.Email)
	}
}

func TestIntegration_Update(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{Email: "update@example.com", PasswordHash: strPtr("hash"), Name: "Original Name", Role: "user"}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("setup: failed to create user: %v", err)
	}

	created.Name = "Updated Name"
	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("expected name %q, got %q", "Updated Name", updated.Name)
	}

	// verify persisted
	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error on re-fetch, got %v", err)
	}
	if found.Name != "Updated Name" {
		t.Errorf("expected persisted name %q, got %q", "Updated Name", found.Name)
	}
}
