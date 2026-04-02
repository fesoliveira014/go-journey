package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
)

type seedBook struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	ISBN          string `json:"isbn"`
	Genre         string `json:"genre"`
	Description   string `json:"description"`
	PublishedYear int32  `json:"published_year"`
	TotalCopies   int32  `json:"total_copies"`
}

func main() {
	authAddr := flag.String("auth-addr", "localhost:50051", "auth service gRPC address")
	catalogAddr := flag.String("catalog-addr", "localhost:50052", "catalog service gRPC address")
	email := flag.String("email", "", "admin email (required)")
	password := flag.String("password", "", "admin password (required)")
	booksFile := flag.String("books", "services/catalog/cmd/seed/books.json", "path to books JSON file")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: seed --email EMAIL --password PASSWORD [--auth-addr ADDR] [--catalog-addr ADDR] [--books FILE]")
		os.Exit(1)
	}

	data, err := os.ReadFile(*booksFile)
	if err != nil {
		log.Fatalf("failed to read books file: %v", err)
	}
	var books []seedBook
	if err := json.Unmarshal(data, &books); err != nil {
		log.Fatalf("failed to parse books JSON: %v", err)
	}

	authConn, err := grpc.NewClient(*authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to auth service: %v", err)
	}
	defer authConn.Close()

	authClient := authv1.NewAuthServiceClient(authConn)
	loginResp, err := authClient.Login(context.Background(), &authv1.LoginRequest{
		Email: *email, Password: *password,
	})
	if err != nil {
		log.Fatalf("login failed: %v", err)
	}
	fmt.Println("Logged in successfully")

	catalogConn, err := grpc.NewClient(*catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to catalog service: %v", err)
	}
	defer catalogConn.Close()

	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+loginResp.Token)

	created, skipped := 0, 0
	for _, b := range books {
		_, err := catalogClient.CreateBook(ctx, &catalogv1.CreateBookRequest{
			Title: b.Title, Author: b.Author, Isbn: b.ISBN,
			Genre: b.Genre, Description: b.Description,
			PublishedYear: b.PublishedYear, TotalCopies: b.TotalCopies,
		})
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code() == codes.AlreadyExists {
				fmt.Printf("  skipped (exists): %s\n", b.Title)
				skipped++
				continue
			}
			log.Fatalf("failed to create book %q: %v", b.Title, err)
		}
		fmt.Printf("  created: %s\n", b.Title)
		created++
	}
	fmt.Printf("\nDone: %d created, %d skipped\n", created, skipped)
}
