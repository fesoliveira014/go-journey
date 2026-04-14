package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/reservation/internal/model"
	"github.com/fesoliveira014/library-system/services/reservation/internal/repository"
	"github.com/fesoliveira014/library-system/services/reservation/migrations"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}

	// Run the same versioned SQL migrations production uses. Unlike GORM's
	// AutoMigrate, these apply CHECK constraints, UNIQUE indexes, and
	// defaults that live only in the SQL files — tests then catch the same
	// database-enforced invariants that production does.
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		t.Fatalf("create migration driver: %v", err)
	}
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("create migration source: %v", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("apply migrations: %v", err)
	}

	t.Cleanup(func() {
		if _, err := sqlDB.Exec("DELETE FROM reservations"); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return db
}

func TestCreate(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewReservationRepository(db)

	r := &model.Reservation{
		UserID:     uuid.New(),
		BookID:     uuid.New(),
		Status:     model.StatusActive,
		ReservedAt: time.Now(),
		DueAt:      time.Now().Add(14 * 24 * time.Hour),
	}

	created, err := repo.Create(context.Background(), r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
}

func TestCountActive(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewReservationRepository(db)
	userID := uuid.New()

	for i := 0; i < 3; i++ {
		if _, err := repo.Create(context.Background(), &model.Reservation{
			UserID: userID, BookID: uuid.New(),
			Status: model.StatusActive, ReservedAt: time.Now(),
			DueAt: time.Now().Add(14 * 24 * time.Hour),
		}); err != nil {
			t.Fatalf("setup: create active reservation: %v", err)
		}
	}
	// One returned reservation — should not count
	if _, err := repo.Create(context.Background(), &model.Reservation{
		UserID: userID, BookID: uuid.New(),
		Status: model.StatusReturned, ReservedAt: time.Now(),
		DueAt: time.Now().Add(14 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("setup: create returned reservation: %v", err)
	}

	count, err := repo.CountActive(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 active, got %d", count)
	}
}

func TestGetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewReservationRepository(db)

	r := &model.Reservation{
		UserID: uuid.New(), BookID: uuid.New(),
		Status: model.StatusActive, ReservedAt: time.Now(),
		DueAt: time.Now().Add(14 * 24 * time.Hour),
	}
	created, _ := repo.Create(context.Background(), r)

	found, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, found.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewReservationRepository(db)

	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != model.ErrReservationNotFound {
		t.Errorf("expected ErrReservationNotFound, got %v", err)
	}
}

func TestListByUser(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewReservationRepository(db)
	userID := uuid.New()

	for i := 0; i < 2; i++ {
		if _, err := repo.Create(context.Background(), &model.Reservation{
			UserID: userID, BookID: uuid.New(),
			Status: model.StatusActive, ReservedAt: time.Now(),
			DueAt: time.Now().Add(14 * 24 * time.Hour),
		}); err != nil {
			t.Fatalf("setup: create reservation: %v", err)
		}
	}
	// Different user
	if _, err := repo.Create(context.Background(), &model.Reservation{
		UserID: uuid.New(), BookID: uuid.New(),
		Status: model.StatusActive, ReservedAt: time.Now(),
		DueAt: time.Now().Add(14 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("setup: create other user reservation: %v", err)
	}

	list, err := repo.ListByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 reservations, got %d", len(list))
	}
}

func TestUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewReservationRepository(db)

	r := &model.Reservation{
		UserID: uuid.New(), BookID: uuid.New(),
		Status: model.StatusActive, ReservedAt: time.Now(),
		DueAt: time.Now().Add(14 * 24 * time.Hour),
	}
	created, _ := repo.Create(context.Background(), r)

	now := time.Now()
	created.Status = model.StatusReturned
	created.ReturnedAt = &now

	updated, err := repo.Update(context.Background(), created)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != model.StatusReturned {
		t.Errorf("expected status returned, got %s", updated.Status)
	}
}
