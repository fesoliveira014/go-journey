package repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

// testDB returns a GORM connection to a test PostgreSQL database.
// Set TEST_DATABASE_URL env var or it defaults to localhost.
// These are integration tests — they require a running PostgreSQL.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=postgres password=postgres dbname=catalog_test sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to PostgreSQL: %v", err)
	}

	// Run the real migrations (same as production) so CHECK constraints and
	// indexes exist in the test database.
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

	// Clean table before each test
	db.Exec("TRUNCATE TABLE books CASCADE")

	return db
}

func newTestBook(suffix string) *model.Book {
	return &model.Book{
		Title:           fmt.Sprintf("Test Book %s", suffix),
		Author:          fmt.Sprintf("Author %s", suffix),
		ISBN:            fmt.Sprintf("978000000%s", suffix[:4]),
		Genre:           "Testing",
		Description:     "A test book",
		PublishedYear:   2024,
		TotalCopies:     3,
		AvailableCopies: 3,
	}
}

func TestBookRepository_Create(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0001")
	created, err := repo.Create(ctx, book)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected UUID to be set")
	}
	if created.Title != book.Title {
		t.Errorf("expected title %q, got %q", book.Title, created.Title)
	}
}

func TestBookRepository_Create_DuplicateISBN(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book1 := newTestBook("0002")
	if _, err := repo.Create(ctx, book1); err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	book2 := newTestBook("0003")
	book2.ISBN = book1.ISBN // same ISBN
	_, err := repo.Create(ctx, book2)
	if err != model.ErrDuplicateISBN {
		t.Errorf("expected ErrDuplicateISBN, got %v", err)
	}
}

func TestBookRepository_GetByID(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0004")
	created, _ := repo.Create(ctx, book)

	found, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Title != created.Title {
		t.Errorf("expected title %q, got %q", created.Title, found.Title)
	}
}

func TestBookRepository_GetByID_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if err != model.ErrBookNotFound {
		t.Errorf("expected ErrBookNotFound, got %v", err)
	}
}

func TestBookRepository_Update(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0005")
	created, _ := repo.Create(ctx, book)

	created.Title = "Updated Title"
	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("expected title %q, got %q", "Updated Title", updated.Title)
	}
}

func TestBookRepository_Delete(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0006")
	created, _ := repo.Create(ctx, book)

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err := repo.GetByID(ctx, created.ID)
	if err != model.ErrBookNotFound {
		t.Errorf("expected ErrBookNotFound after delete, got %v", err)
	}
}

func TestBookRepository_Delete_NotFound(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New())
	if err != model.ErrBookNotFound {
		t.Errorf("expected ErrBookNotFound, got %v", err)
	}
}

func TestBookRepository_List(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	// Create books with different genres
	for i := 0; i < 3; i++ {
		b := newTestBook(fmt.Sprintf("%04d", 100+i))
		b.Genre = "Fiction"
		repo.Create(ctx, b)
	}
	for i := 0; i < 2; i++ {
		b := newTestBook(fmt.Sprintf("%04d", 200+i))
		b.Genre = "Science"
		repo.Create(ctx, b)
	}

	// List all
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

	// Filter by genre
	books, total, err = repo.List(ctx, model.BookFilter{Genre: "Fiction"}, model.Pagination{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3 for Fiction, got %d", total)
	}
}

func TestBookRepository_UpdateAvailability(t *testing.T) {
	db := testDB(t)
	repo := repository.NewBookRepository(db)
	ctx := context.Background()

	book := newTestBook("0007")
	book.TotalCopies = 5
	book.AvailableCopies = 5
	created, _ := repo.Create(ctx, book)

	// Decrement
	if err := repo.UpdateAvailability(ctx, created.ID, -1); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	found, _ := repo.GetByID(ctx, created.ID)
	if found.AvailableCopies != 4 {
		t.Errorf("expected 4 available copies, got %d", found.AvailableCopies)
	}
}
