package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// TODO(Task 6): wire gRPC clients and parsed templates; register all routes.
	srv := handler.New(nil, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", srv.Health)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
