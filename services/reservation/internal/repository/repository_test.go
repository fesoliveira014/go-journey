package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/reservation/internal/model"
	"github.com/fesoliveira014/library-system/services/reservation/internal/repository"
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
	sqlDB, _ := db.DB()
	sqlDB.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")
	db.AutoMigrate(&model.Reservation{})
	t.Cleanup(func() {
		sqlDB.Exec("DELETE FROM reservations")
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
		repo.Create(context.Background(), &model.Reservation{
			UserID: userID, BookID: uuid.New(),
			Status: model.StatusActive, ReservedAt: time.Now(),
			DueAt: time.Now().Add(14 * 24 * time.Hour),
		})
	}
	// One returned reservation — should not count
	repo.Create(context.Background(), &model.Reservation{
		UserID: userID, BookID: uuid.New(),
		Status: model.StatusReturned, ReservedAt: time.Now(),
		DueAt: time.Now().Add(14 * 24 * time.Hour),
	})

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
		repo.Create(context.Background(), &model.Reservation{
			UserID: userID, BookID: uuid.New(),
			Status: model.StatusActive, ReservedAt: time.Now(),
			DueAt: time.Now().Add(14 * 24 * time.Hour),
		})
	}
	// Different user
	repo.Create(context.Background(), &model.Reservation{
		UserID: uuid.New(), BookID: uuid.New(),
		Status: model.StatusActive, ReservedAt: time.Now(),
		DueAt: time.Now().Add(14 * 24 * time.Hour),
	})

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
