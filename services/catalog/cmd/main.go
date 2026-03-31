package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	pkgotel "github.com/fesoliveira014/library-system/pkg/otel"
	"github.com/fesoliveira014/library-system/services/catalog/internal/consumer"
	"github.com/fesoliveira014/library-system/services/catalog/internal/handler"
	catalogkafka "github.com/fesoliveira014/library-system/services/catalog/internal/kafka"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

// noopPublisher is used when KAFKA_BROKERS is not configured.
type noopPublisher struct{}

func (n *noopPublisher) Publish(_ context.Context, _ service.BookEvent) error { return nil }

func main() {
	otelCtx := context.Background()
	shutdown, err := pkgotel.Init(otelCtx, "catalog", os.Getenv("OTEL_COLLECTOR_ENDPOINT"))
	if err != nil {
		slog.Error("failed to init otel", "error", err)
	} else {
		defer shutdown(otelCtx)
	}

	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		dbDSN = "host=localhost port=5432 user=postgres password=postgres dbname=catalog sslmode=disable"
	}
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50052"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-in-production"
	}
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")

	db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to PostgreSQL")

	if err := db.Use(tracing.NewPlugin()); err != nil {
		slog.Error("failed to add otel gorm plugin", "error", err)
	}

	if err := runMigrations(db); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations completed")

	bookRepo := repository.NewBookRepository(db)

	var publisher service.EventPublisher = &noopPublisher{}
	var brokers []string
	if kafkaBrokers != "" {
		brokers = strings.Split(kafkaBrokers, ",")
		pub, err := catalogkafka.NewPublisher(brokers, "catalog.books.changed")
		if err != nil {
			slog.Error("failed to create kafka publisher", "error", err)
			os.Exit(1)
		}
		defer pub.Close()
		publisher = pub
		slog.Info("kafka publisher initialized", "topic", "catalog.books.changed")
	}

	catalogSvc := service.NewCatalogService(bookRepo, publisher)
	catalogHandler := handler.NewCatalogHandler(catalogSvc)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if len(brokers) > 0 {
		go func() {
			slog.Info("starting kafka consumer", "topic", "reservations")
			if err := consumer.Run(ctx, brokers, "reservations", catalogSvc); err != nil {
				slog.Error("kafka consumer error", "error", err)
			}
		}()
	}

	skipMethods := []string{
		"/catalog.v1.CatalogService/GetBook",
		"/catalog.v1.CatalogService/ListBooks",
		"/catalog.v1.CatalogService/UpdateAvailability",
	}
	interceptor := pkgauth.UnaryAuthInterceptor(jwtSecret, skipMethods)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(interceptor),
	)
	catalogv1.RegisterCatalogServiceServer(grpcServer, catalogHandler)
	reflection.Register(grpcServer)

	slog.Info("catalog service listening", "port", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}

func runMigrations(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
