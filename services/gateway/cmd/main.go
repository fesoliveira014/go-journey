package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	pkgotel "github.com/fesoliveira014/library-system/pkg/otel"
	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
	"github.com/fesoliveira014/library-system/services/gateway/internal/middleware"
)

func main() {
	ctx := context.Background()
	shutdown, err := pkgotel.Init(ctx, "gateway", "0.1.0", os.Getenv("OTEL_COLLECTOR_ENDPOINT"))
	if err != nil {
		slog.Error("failed to init otel", "error", err)
	} else {
		defer func() { _ = shutdown(ctx) }()
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET is required")
		os.Exit(1)
	}

	authAddr := os.Getenv("AUTH_GRPC_ADDR")
	if authAddr == "" {
		authAddr = "localhost:50051"
	}
	authConn, err := grpc.NewClient(authAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("connect to auth service", "error", err)
		os.Exit(1)
	}
	defer authConn.Close()

	catalogAddr := os.Getenv("CATALOG_GRPC_ADDR")
	if catalogAddr == "" {
		catalogAddr = "localhost:50052"
	}
	catalogConn, err := grpc.NewClient(catalogAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithUnaryInterceptor(pkgauth.UnaryForwardAuthInterceptor()),
	)
	if err != nil {
		slog.Error("connect to catalog service", "error", err)
		os.Exit(1)
	}
	defer catalogConn.Close()

	reservationAddr := os.Getenv("RESERVATION_GRPC_ADDR")
	if reservationAddr == "" {
		reservationAddr = "localhost:50053"
	}
	reservationConn, err := grpc.NewClient(reservationAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithUnaryInterceptor(pkgauth.UnaryForwardAuthInterceptor()),
	)
	if err != nil {
		slog.Error("connect to reservation service", "error", err)
		os.Exit(1)
	}
	defer reservationConn.Close()

	searchAddr := os.Getenv("SEARCH_GRPC_ADDR")
	if searchAddr == "" {
		searchAddr = "localhost:50054"
	}
	searchConn, err := grpc.NewClient(searchAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithUnaryInterceptor(pkgauth.UnaryForwardAuthInterceptor()),
	)
	if err != nil {
		slog.Error("connect to search service", "error", err)
		os.Exit(1)
	}
	defer searchConn.Close()

	tmpl, err := handler.ParseTemplates("templates")
	if err != nil {
		slog.Error("parse templates", "error", err)
		os.Exit(1)
	}

	authClient := authv1.NewAuthServiceClient(authConn)
	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)
	reservationClient := reservationv1.NewReservationServiceClient(reservationConn)
	searchClient := searchv1.NewSearchServiceClient(searchConn)
	srv := handler.New(authClient, catalogClient, reservationClient, searchClient, tmpl)

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("GET /healthz", srv.Health)
	mux.HandleFunc("GET /{$}", srv.Home)

	mux.HandleFunc("GET /login", srv.LoginPage)
	mux.HandleFunc("POST /login", srv.LoginSubmit)
	mux.HandleFunc("GET /register", srv.RegisterPage)
	mux.HandleFunc("POST /register", srv.RegisterSubmit)
	mux.HandleFunc("POST /logout", srv.Logout)
	mux.HandleFunc("GET /auth/oauth2/google", srv.OAuth2Start)
	mux.HandleFunc("GET /auth/oauth2/google/callback", srv.OAuth2Callback)

	mux.HandleFunc("GET /books", srv.BookList)
	mux.HandleFunc("GET /books/{id}", srv.BookDetail)

	mux.HandleFunc("POST /books/{id}/reserve", srv.ReserveBook)
	mux.HandleFunc("GET /reservations", srv.MyReservations)
	mux.HandleFunc("POST /reservations/{id}/return", srv.ReturnBook)

	mux.HandleFunc("GET /search", srv.SearchPage)
	mux.HandleFunc("GET /search/suggest", srv.SearchSuggest)

	mux.HandleFunc("GET /admin", srv.AdminDashboard)
	mux.HandleFunc("GET /admin/users", srv.AdminUserList)
	mux.HandleFunc("GET /admin/reservations", srv.AdminReservationList)

	mux.HandleFunc("GET /admin/books/new", srv.AdminBookNew)
	mux.HandleFunc("POST /admin/books", srv.AdminBookCreate)
	mux.HandleFunc("GET /admin/books/{id}/edit", srv.AdminBookEdit)
	mux.HandleFunc("POST /admin/books/{id}", srv.AdminBookUpdate)
	mux.HandleFunc("POST /admin/books/{id}/delete", srv.AdminBookDelete)

	var h http.Handler = mux
	h = middleware.Auth(h, jwtSecret)
	h = middleware.Logging(h)
	h = otelhttp.NewHandler(h, "gateway")

	var cancel context.CancelFunc
	ctx, cancel = signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	addr := fmt.Sprintf(":%s", port)

	server := &http.Server{
		Addr:    addr,
		Handler: h,
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down gateway")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("gateway shutdown error", "error", err)
		}
	}()

	slog.Info("gateway listening", "addr", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
