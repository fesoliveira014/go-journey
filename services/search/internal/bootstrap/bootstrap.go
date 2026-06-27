package bootstrap

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

// IndexBootstrapper is the subset of SearchService that bootstrap needs.
type IndexBootstrapper interface {
	EnsureIndex(ctx context.Context) error
	ResetIndex(ctx context.Context) error
	Count(ctx context.Context) (int64, error)
	Upsert(ctx context.Context, doc model.BookDocument) error
}

type Mode string

const (
	ModeIfEmpty  Mode = "if_empty"
	ModeAlways   Mode = "always"
	ModeDisabled Mode = "disabled"
)

func ParseMode(value string) (Mode, error) {
	switch Mode(strings.TrimSpace(value)) {
	case "":
		return ModeIfEmpty, nil
	case ModeIfEmpty:
		return ModeIfEmpty, nil
	case ModeAlways:
		return ModeAlways, nil
	case ModeDisabled:
		return ModeDisabled, nil
	default:
		return "", fmt.Errorf("unknown search bootstrap mode %q", value)
	}
}

// Run syncs the Meilisearch index from the Catalog service according to mode.
func Run(ctx context.Context, catalog catalogv1.CatalogServiceClient, svc IndexBootstrapper, mode Mode) error {
	if mode == ModeAlways {
		log.Println("resetting search index before bootstrap...")
		if err := svc.ResetIndex(ctx); err != nil {
			return err
		}
	}

	if err := svc.EnsureIndex(ctx); err != nil {
		return err
	}

	if mode == ModeDisabled {
		log.Println("search bootstrap disabled; ensured index only")
		return nil
	}

	count, err := svc.Count(ctx)
	if err != nil {
		return err
	}
	if mode == ModeIfEmpty && count > 0 {
		log.Printf("search index already has %d documents, skipping bootstrap", count)
		return nil
	}

	log.Println("bootstrapping search index from catalog...")

	// Retry the first ListBooks call — catalog may still be starting.
	var firstResp *catalogv1.ListBooksResponse
	for attempt := 1; attempt <= 10; attempt++ {
		var err error
		firstResp, err = catalog.ListBooks(ctx, &catalogv1.ListBooksRequest{
			Page:     1,
			PageSize: 100,
		})
		if err == nil {
			break
		}
		if attempt == 10 {
			return fmt.Errorf("catalog not reachable after %d attempts: %w", attempt, err)
		}
		log.Printf("bootstrap: catalog not ready (attempt %d/10), retrying in 3s: %v", attempt, err)
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	var page int32 = 1
	var total int64

	// Process the first page we already fetched.
	resp := firstResp
	for {

		for _, b := range resp.Books {
			doc := model.BookDocument{
				ID:              b.Id,
				Title:           b.Title,
				Author:          b.Author,
				ISBN:            b.Isbn,
				Genre:           b.Genre,
				Description:     b.Description,
				PublishedYear:   int(b.PublishedYear),
				TotalCopies:     int(b.TotalCopies),
				AvailableCopies: int(b.AvailableCopies),
			}
			if err := svc.Upsert(ctx, doc); err != nil {
				log.Printf("failed to index book %s: %v", b.Id, err)
			}
			total++
		}

		if total%100 == 0 && total > 0 {
			log.Printf("bootstrap progress: %d books indexed", total)
		}

		if len(resp.Books) < 100 {
			break
		}
		page++

		var err error
		resp, err = catalog.ListBooks(ctx, &catalogv1.ListBooksRequest{
			Page:     page,
			PageSize: 100,
		})
		if err != nil {
			return err
		}
	}

	log.Printf("bootstrap complete: %d books indexed", total)
	return nil
}
