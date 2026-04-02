package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	pkgotel "github.com/fesoliveira014/library-system/pkg/otel"
	"github.com/fesoliveira014/library-system/services/reservation/internal/handler"
	"github.com/fesoliveira014/library-system/services/reservation/internal/kafka"
	"github.com/fesoliveira014/library-system/services/reservation/internal/repository"
	"github.com/fesoliveira014/library-system/services/reservation/internal/service"
	"github.com/fesoliveira014/library-system/services/reservation/migrations"
)

func main() {
	otelCtx := context.Background()
	shutdown, err := pkgotel.Init(otelCtx, "reservation", "0.1.0", os.Getenv("OTEL_COLLECTOR_ENDPOINT"))
	if err != nil {
		slog.Error("failed to init otel", "error", err)
	} else {
		defer func() { _ = shutdown(otelCtx) }()
	}

	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		dbDSN = "host=localhost port=5435 user=postgres password=postgres dbname=reservation sslmode=disable"
	}
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50053"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-in-production"
	}
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}
	catalogAddr := os.Getenv("CATALOG_GRPC_ADDR")
	if catalogAddr == "" {
		catalogAddr = "localhost:50052"
	}
	maxActiveStr := os.Getenv("MAX_ACTIVE_RESERVATIONS")
	maxActive := 5
	if maxActiveStr != "" {
		if v, err := strconv.Atoi(maxActiveStr); err == nil {
			maxActive = v
		}
	}

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

	catalogConn, err := grpc.NewClient(catalogAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("connect to catalog service", "error", err)
		os.Exit(1)
	}
	defer catalogConn.Close()
	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)

	brokers := strings.Split(kafkaBrokers, ",")
	publisher, err := kafka.NewPublisher(brokers, "reservations")
	if err != nil {
		slog.Error("create kafka publisher", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	repo := repository.NewReservationRepository(db)
	reservationSvc := service.NewReservationService(repo, catalogClient, publisher, maxActive)
	reservationHandler := handler.NewReservationHandler(reservationSvc)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	interceptor := pkgauth.UnaryAuthInterceptor(jwtSecret, nil)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(interceptor),
	)
	reservationv1.RegisterReservationServiceServer(grpcServer, reservationHandler)
	reflection.Register(grpcServer)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	go func() {
		<-ctx.Done()
		slog.Info("shutting down reservation service")
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		grpcServer.GracefulStop()
	}()

	slog.Info("reservation service listening", "port", grpcPort)
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
