package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/catalog/internal/consumer"
	"github.com/fesoliveira014/library-system/services/catalog/internal/handler"
	"github.com/fesoliveira014/library-system/services/catalog/internal/repository"
	"github.com/fesoliveira014/library-system/services/catalog/internal/service"
	"github.com/fesoliveira014/library-system/services/catalog/migrations"
)

func main() {
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
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("connected to PostgreSQL")

	if err := runMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations completed")

	bookRepo := repository.NewBookRepository(db)
	catalogSvc := service.NewCatalogService(bookRepo)
	catalogHandler := handler.NewCatalogHandler(catalogSvc)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if kafkaBrokers != "" {
		brokers := strings.Split(kafkaBrokers, ",")
		go func() {
			log.Println("starting kafka consumer for reservations topic")
			if err := consumer.Run(ctx, brokers, "reservations", catalogSvc); err != nil {
				log.Printf("kafka consumer error: %v", err)
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
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	catalogv1.RegisterCatalogServiceServer(grpcServer, catalogHandler)
	reflection.Register(grpcServer)

	log.Printf("catalog service listening on :%s", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
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
