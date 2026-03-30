package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"github.com/fesoliveira014/library-system/services/auth/internal/handler"
	"github.com/fesoliveira014/library-system/services/auth/internal/repository"
	"github.com/fesoliveira014/library-system/services/auth/internal/service"
	"github.com/fesoliveira014/library-system/services/auth/migrations"
)

func main() {
	// Configuration from environment
	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		dbDSN = "host=localhost port=5434 user=postgres password=postgres dbname=auth sslmode=disable"
	}
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-in-production"
	}
	jwtExpiry := os.Getenv("JWT_EXPIRY")
	if jwtExpiry == "" {
		jwtExpiry = "24h"
	}
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	// Connect to PostgreSQL via GORM
	db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("connected to PostgreSQL")

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations completed")

	// Wire dependencies
	userRepo := repository.NewUserRepository(db)
	authSvc := service.NewAuthService(userRepo, jwtSecret, jwtExpiry)
	authHandler := handler.NewAuthHandlerWithOAuth(authSvc, googleClientID, googleClientSecret, googleRedirectURL)

	// Start gRPC server with auth interceptor
	skipMethods := []string{
		"/auth.v1.AuthService/Register",
		"/auth.v1.AuthService/Login",
		"/auth.v1.AuthService/ValidateToken",
		"/auth.v1.AuthService/InitOAuth2",
		"/auth.v1.AuthService/CompleteOAuth2",
	}
	interceptor := pkgauth.UnaryAuthInterceptor(jwtSecret, skipMethods)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	authv1.RegisterAuthServiceServer(grpcServer, authHandler)
	reflection.Register(grpcServer)

	log.Printf("auth service listening on :%s", grpcPort)
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
