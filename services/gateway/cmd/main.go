package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
	"github.com/fesoliveira014/library-system/services/gateway/internal/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	authAddr := os.Getenv("AUTH_GRPC_ADDR")
	if authAddr == "" {
		authAddr = "localhost:50051"
	}
	authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to auth service: %v", err)
	}
	defer authConn.Close()

	catalogAddr := os.Getenv("CATALOG_GRPC_ADDR")
	if catalogAddr == "" {
		catalogAddr = "localhost:50052"
	}
	catalogConn, err := grpc.NewClient(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to catalog service: %v", err)
	}
	defer catalogConn.Close()

	reservationAddr := os.Getenv("RESERVATION_GRPC_ADDR")
	if reservationAddr == "" {
		reservationAddr = "localhost:50053"
	}
	reservationConn, err := grpc.NewClient(reservationAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to reservation service: %v", err)
	}
	defer reservationConn.Close()

	tmpl, err := handler.ParseTemplates("templates")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	authClient := authv1.NewAuthServiceClient(authConn)
	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)
	reservationClient := reservationv1.NewReservationServiceClient(reservationConn)
	srv := handler.New(authClient, catalogClient, reservationClient, tmpl)

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

	mux.HandleFunc("GET /admin/books/new", srv.AdminBookNew)
	mux.HandleFunc("POST /admin/books", srv.AdminBookCreate)
	mux.HandleFunc("GET /admin/books/{id}/edit", srv.AdminBookEdit)
	mux.HandleFunc("POST /admin/books/{id}", srv.AdminBookUpdate)
	mux.HandleFunc("POST /admin/books/{id}/delete", srv.AdminBookDelete)

	var h http.Handler = mux
	h = middleware.Auth(h, jwtSecret)
	h = middleware.Logging(h)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
