//go:build integration

package repository_test

import (
	"context"
	"fmt"
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

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

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

func TestIntegration_Create(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("1001")
	created, err := repo.Create(ctx, book)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
}

func TestIntegration_DuplicateISBN(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book1 := newTestBook("2001")
	if _, err := repo.Create(ctx, book1); err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	book2 := newTestBook("2002")
	book2.ISBN = book1.ISBN
	_, err := repo.Create(ctx, book2)
	if err != model.ErrDuplicateISBN {
		t.Errorf("expected ErrDuplicateISBN, got %v", err)
	}
}

func TestIntegration_GetByID(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("3001")
	created, err := repo.Create(ctx, book)
	if err != nil {
		t.Fatalf("setup: create book: %v", err)
	}

	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Title != created.Title {
		t.Errorf("expected title %q, got %q", created.Title, found.Title)
	}
	if found.Author != created.Author {
		t.Errorf("expected author %q, got %q", created.Author, found.Author)
	}
	if found.ISBN != created.ISBN {
		t.Errorf("expected ISBN %q, got %q", created.ISBN, found.ISBN)
	}
}

func TestIntegration_GetByID_NotFound(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if err != model.ErrBookNotFound {
		t.Errorf("expected ErrBookNotFound, got %v", err)
	}
}

func TestIntegration_Update(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("4001")
	created, err := repo.Create(ctx, book)
	if err != nil {
		t.Fatalf("setup: create book: %v", err)
	}

	created.Title = "Integration Updated Title"
	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Title != "Integration Updated Title" {
		t.Errorf("expected title %q, got %q", "Integration Updated Title", updated.Title)
	}

	// Verify persisted
	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error on get, got %v", err)
	}
	if found.Title != "Integration Updated Title" {
		t.Errorf("expected persisted title %q, got %q", "Integration Updated Title", found.Title)
	}
}

func TestIntegration_Delete(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("5001")
	created, err := repo.Create(ctx, book)
	if err != nil {
		t.Fatalf("setup: create book: %v", err)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("expected no error on delete, got %v", err)
	}

	_, err = repo.GetByID(ctx, created.ID)
	if err != model.ErrBookNotFound {
		t.Errorf("expected ErrBookNotFound after delete, got %v", err)
	}
}

func TestIntegration_List(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		b := newTestBook(fmt.Sprintf("%04d", 6100+i))
		b.Genre = "Fiction"
		if _, err := repo.Create(ctx, b); err != nil {
			t.Fatalf("setup: create fiction book: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		b := newTestBook(fmt.Sprintf("%04d", 6200+i))
		b.Genre = "Science"
		if _, err := repo.Create(ctx, b); err != nil {
			t.Fatalf("setup: create science book: %v", err)
		}
	}

	books, total, err := repo.List(ctx, model.BookFilter{}, model.Pagination{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(books) != 5 {
		t.Errorf("expected 5 books, got %d", len(books))
	}

	books, total, err = repo.List(ctx, model.BookFilter{Genre: "Fiction"}, model.Pagination{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("expected no error on genre filter, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3 for Fiction, got %d", total)
	}
	if len(books) != 3 {
		t.Errorf("expected 3 fiction books, got %d", len(books))
	}
}

func TestIntegration_UpdateAvailability(t *testing.T) {
	db := setupPostgres(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("7001")
	book.TotalCopies = 5
	book.AvailableCopies = 5
	created, err := repo.Create(ctx, book)
	if err != nil {
		t.Fatalf("setup: create book: %v", err)
	}

	if err := repo.UpdateAvailability(ctx, created.ID, -1); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error on get, got %v", err)
	}
	if found.AvailableCopies != 4 {
		t.Errorf("expected 4 available copies, got %d", found.AvailableCopies)
	}
}
