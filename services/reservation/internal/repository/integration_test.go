//go:build integration

package repository_test

import (
	"context"
	"database/sql"
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

	"github.com/fesoliveira014/library-system/services/reservation/internal/model"
	"github.com/fesoliveira014/library-system/services/reservation/internal/repository"
	"github.com/fesoliveira014/library-system/services/reservation/migrations"
)

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
			t.Logf("failed to terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm connection: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}

	if err := runMigrations(t, sqlDB); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	if err := db.Exec("TRUNCATE TABLE reservations").Error; err != nil {
		t.Fatalf("failed to truncate reservations: %v", err)
	}

	return db
}

func runMigrations(t *testing.T, sqlDB *sql.DB) error {
	t.Helper()

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", src, "reservation_test", driver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

func newTestReservation(userID, bookID uuid.UUID) *model.Reservation {
	return &model.Reservation{
		UserID:     userID,
		BookID:     bookID,
		Status:     model.StatusActive,
		ReservedAt: time.Now(),
		DueAt:      time.Now().Add(14 * 24 * time.Hour),
	}
}

func TestIntegration_Create(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewReservationRepository(db)
	ctx := context.Background()

	res := newTestReservation(uuid.New(), uuid.New())
	created, err := repo.Create(ctx, res)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected non-nil UUID after create")
	}
}

func TestIntegration_CountActive(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewReservationRepository(db)
	ctx := context.Background()

	userID := uuid.New()

	for i := 0; i < 3; i++ {
		res := newTestReservation(userID, uuid.New())
		if _, err := repo.Create(ctx, res); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	returned := newTestReservation(userID, uuid.New())
	returned.Status = model.StatusReturned
	if _, err := repo.Create(ctx, returned); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	count, err := repo.CountActive(ctx, userID)
	if err != nil {
		t.Fatalf("CountActive() error = %v", err)
	}
	if count != 3 {
		t.Errorf("CountActive() = %d, want 3", count)
	}
}

func TestIntegration_GetByID(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewReservationRepository(db)
	ctx := context.Background()

	res := newTestReservation(uuid.New(), uuid.New())
	created, err := repo.Create(ctx, res)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetByID() ID = %v, want %v", got.ID, created.ID)
	}
}

func TestIntegration_GetByID_NotFound(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewReservationRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if err != model.ErrReservationNotFound {
		t.Errorf("GetByID() error = %v, want %v", err, model.ErrReservationNotFound)
	}
}

func TestIntegration_ListByUser(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewReservationRepository(db)
	ctx := context.Background()

	userA := uuid.New()
	userB := uuid.New()

	for i := 0; i < 2; i++ {
		res := newTestReservation(userA, uuid.New())
		if _, err := repo.Create(ctx, res); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	resB := newTestReservation(userB, uuid.New())
	if _, err := repo.Create(ctx, resB); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	list, err := repo.ListByUser(ctx, userA)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListByUser() count = %d, want 2", len(list))
	}
}

func TestIntegration_Update(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewReservationRepository(db)
	ctx := context.Background()

	res := newTestReservation(uuid.New(), uuid.New())
	created, err := repo.Create(ctx, res)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	created.Status = model.StatusReturned
	now := time.Now()
	created.ReturnedAt = &now

	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != model.StatusReturned {
		t.Errorf("Update() Status = %q, want %q", updated.Status, model.StatusReturned)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() after update error = %v", err)
	}
	if got.Status != model.StatusReturned {
		t.Errorf("persisted Status = %q, want %q", got.Status, model.StatusReturned)
	}
}
