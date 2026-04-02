package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/bootstrap"
	"github.com/fesoliveira014/library-system/services/search/internal/consumer"
	"github.com/fesoliveira014/library-system/services/search/internal/handler"
	"github.com/fesoliveira014/library-system/services/search/internal/index"
	"github.com/fesoliveira014/library-system/services/search/internal/service"
)

func main() {
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50054"
	}
	meiliURL := os.Getenv("MEILI_URL")
	if meiliURL == "" {
		meiliURL = "http://localhost:7700"
	}
	meiliKey := os.Getenv("MEILI_MASTER_KEY")
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	catalogAddr := os.Getenv("CATALOG_GRPC_ADDR")
	if catalogAddr == "" {
		catalogAddr = "localhost:50052"
	}

	idx := index.NewMeilisearchIndex(meiliURL, meiliKey)
	searchSvc := service.NewSearchService(idx)

	// Bootstrap: connect to catalog and sync if index is empty
	catalogConn, err := grpc.NewClient(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to catalog service: %v", err)
	}
	defer catalogConn.Close()

	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := bootstrap.Run(ctx, catalogClient, searchSvc); err != nil {
		log.Printf("bootstrap failed (starting with empty index): %v", err)
	}

	// Start Kafka consumer
	if kafkaBrokers != "" {
		brokers := strings.Split(kafkaBrokers, ",")
		go func() {
			log.Println("starting kafka consumer for catalog.books.changed topic")
			if err := consumer.Run(ctx, brokers, "catalog.books.changed", searchSvc); err != nil {
				log.Printf("kafka consumer error: %v", err)
			}
		}()
	}

	// Start gRPC server
	searchHandler := handler.NewSearchHandler(searchSvc)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	searchv1.RegisterSearchServiceServer(grpcServer, searchHandler)
	reflection.Register(grpcServer)
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	go func() {
		<-ctx.Done()
		log.Println("shutting down search service")
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		grpcServer.GracefulStop()
	}()

	log.Printf("search service listening on :%s", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
