package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/auth/internal/model"
	"github.com/fesoliveira014/library-system/services/auth/internal/repository"
	"github.com/fesoliveira014/library-system/services/auth/migrations"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost port=5434 user=postgres password=postgres dbname=auth_test sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to PostgreSQL: %v", err)
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

func strPtr(s string) *string { return &s }

func TestUserRepository_Create(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: strPtr("$2a$10$hashedpassword"),
		Name:         "Test User",
		Role:         "user",
	}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected UUID to be set")
	}
	if created.Email != "test@example.com" {
		t.Errorf("expected email %q, got %q", "test@example.com", created.Email)
	}
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	db := testDB(t)
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

func TestUserRepository_GetByID(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{Email: "get@example.com", PasswordHash: strPtr("hash"), Name: "Get User", Role: "user"}
	created, _ := repo.Create(ctx, user)

	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Email != "get@example.com" {
		t.Errorf("expected email %q, got %q", "get@example.com", found.Email)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if err != model.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{Email: "email@example.com", PasswordHash: strPtr("hash"), Name: "Email User", Role: "user"}
	repo.Create(ctx, user)

	found, err := repo.GetByEmail(ctx, "email@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Name != "Email User" {
		t.Errorf("expected name %q, got %q", "Email User", found.Name)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
	if err != model.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_GetByOAuthID(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	provider := "google"
	oauthID := "google-123"
	user := &model.User{
		Email:         "oauth@example.com",
		Name:          "OAuth User",
		Role:          "user",
		OAuthProvider: &provider,
		OAuthID:       &oauthID,
	}
	repo.Create(ctx, user)

	found, err := repo.GetByOAuthID(ctx, "google", "google-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Email != "oauth@example.com" {
		t.Errorf("expected email %q, got %q", "oauth@example.com", found.Email)
	}
}

func TestUserRepository_GetByOAuthID_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByOAuthID(ctx, "google", "nonexistent")
	if err != model.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := testDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{Email: "update@example.com", PasswordHash: strPtr("hash"), Name: "Original", Role: "user"}
	created, _ := repo.Create(ctx, user)

	created.Name = "Updated"
	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("expected name %q, got %q", "Updated", updated.Name)
	}
}
