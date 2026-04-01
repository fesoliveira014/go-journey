//go:build integration

package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	kafkatc "github.com/testcontainers/testcontainers-go/modules/kafka"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/catalog/internal/consumer"
	"github.com/fesoliveira014/library-system/services/catalog/internal/model"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

type noopPublisher struct{}

func (n *noopPublisher) Publish(_ context.Context, _ service.BookEvent) error { return nil }

func setupKafka(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()

	kafkaContainer, err := kafkatc.Run(ctx, "confluentinc/confluent-local:7.5.0")
	if err != nil {
		t.Fatalf("failed to start kafka container: %v", err)
	}
	t.Cleanup(func() {
		if err := kafkaContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate kafka container: %v", err)
		}
	})

	brokers, err := kafkaContainer.Brokers(ctx)
	if err != nil {
		t.Fatalf("failed to get kafka brokers: %v", err)
	}

	return brokers
}

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

	return db
}

func produceEvent(t *testing.T, brokers []string, topic string, eventType string, bookID uuid.UUID) {
	t.Helper()

	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		t.Fatalf("failed to create producer: %v", err)
	}
	defer producer.Close()

	payload, err := json.Marshal(map[string]string{
		"event_type":     eventType,
		"reservation_id": uuid.New().String(),
		"user_id":        uuid.New().String(),
		"book_id":        bookID.String(),
		"timestamp":      time.Now().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	}

	if _, _, err := producer.SendMessage(msg); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
}

func pollAvailability(t *testing.T, repo *repository.BookRepository, bookID uuid.UUID, expected int) {
	t.Helper()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(10 * time.Second)

	ctx := context.Background()
	for {
		select {
		case <-ticker.C:
			book, err := repo.GetByID(ctx, bookID)
			if err == nil && book.AvailableCopies == expected {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for available_copies == %d", expected)
		}
	}
}

func TestConsumer_Integration_ReservationCreated(t *testing.T) {
	brokers := setupKafka(t)
	db := setupPostgres(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := repository.NewBookRepository(db)
	svc := service.NewCatalogService(repo, &noopPublisher{})

	book := &model.Book{
		Title:           "Integration Test Book",
		Author:          "Test Author",
		ISBN:            "9780000000001",
		TotalCopies:     5,
		AvailableCopies: 5,
	}
	created, err := repo.Create(ctx, book)
	if err != nil {
		t.Fatalf("failed to create book: %v", err)
	}

	go func() {
		if err := consumer.Run(ctx, brokers, "reservations", svc); err != nil {
			t.Logf("consumer exited: %v", err)
		}
	}()

	// Give consumer time to join the group before producing
	time.Sleep(2 * time.Second)

	produceEvent(t, brokers, "reservations", "reservation.created", created.ID)

	pollAvailability(t, repo, created.ID, 4)

	cancel()

	found, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("failed to get book: %v", err)
	}
	if found.AvailableCopies != 4 {
		t.Errorf("expected available_copies == 4, got %d", found.AvailableCopies)
	}
}

func TestConsumer_Integration_ReservationReturned(t *testing.T) {
	brokers := setupKafka(t)
	db := setupPostgres(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := repository.NewBookRepository(db)
	svc := service.NewCatalogService(repo, &noopPublisher{})

	book := &model.Book{
		Title:           "Integration Return Test Book",
		Author:          "Test Author",
		ISBN:            "9780000000002",
		TotalCopies:     5,
		AvailableCopies: 5,
	}
	created, err := repo.Create(ctx, book)
	if err != nil {
		t.Fatalf("failed to create book: %v", err)
	}

	go func() {
		if err := consumer.Run(ctx, brokers, "reservations", svc); err != nil {
			t.Logf("consumer exited: %v", err)
		}
	}()

	// Give consumer time to join the group before producing
	time.Sleep(2 * time.Second)

	// First decrement to 4 via reservation.created
	produceEvent(t, brokers, "reservations", "reservation.created", created.ID)
	pollAvailability(t, repo, created.ID, 4)

	// Then increment back to 5 via reservation.returned
	produceEvent(t, brokers, "reservations", "reservation.returned", created.ID)
	pollAvailability(t, repo, created.ID, 5)

	cancel()

	found, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("failed to get book: %v", err)
	}
	if found.AvailableCopies != 5 {
		t.Errorf("expected available_copies == 5, got %d", found.AvailableCopies)
	}
}
