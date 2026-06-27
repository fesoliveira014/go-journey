package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/bootstrap"
	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/service"
)

func main() {
	meiliURL := os.Getenv("MEILI_URL")
	if meiliURL == "" {
		meiliURL = "http://localhost:7700"
	}
	meiliKey := os.Getenv("MEILI_MASTER_KEY")
	catalogAddr := os.Getenv("CATALOG_GRPC_ADDR")
	if catalogAddr == "" {
		catalogAddr = "localhost:50052"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	catalogConn, err := grpc.NewClient(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to catalog service: %v", err)
	}
	defer catalogConn.Close()

	idx := index.NewMeilisearchIndex(meiliURL, meiliKey)
	searchSvc := service.NewSearchService(idx)
	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)

	if err := bootstrap.Run(ctx, catalogClient, searchSvc, bootstrap.ModeAlways); err != nil {
		log.Fatalf("reindex failed: %v", err)
	}
	log.Println("reindex complete")
}
