package bootstrap

import (
	"context"
	"log"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

// IndexBootstrapper is the subset of SearchService that bootstrap needs.
type IndexBootstrapper interface {
	EnsureIndex(ctx context.Context) error
	Count(ctx context.Context) (int64, error)
	Upsert(ctx context.Context, doc model.BookDocument) error
}

// Run syncs the Meilisearch index from the Catalog service if the index is empty.
func Run(ctx context.Context, catalog catalogv1.CatalogServiceClient, svc IndexBootstrapper) error {
	if err := svc.EnsureIndex(ctx); err != nil {
		return err
	}

	count, err := svc.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("search index already has %d documents, skipping bootstrap", count)
		return nil
	}

	log.Println("search index is empty, bootstrapping from catalog...")

	var page int32 = 1
	var total int64

	for {
		resp, err := catalog.ListBooks(ctx, &catalogv1.ListBooksRequest{
			Page:     page,
			PageSize: 100,
		})
		if err != nil {
			return err
		}

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
	}

	log.Printf("bootstrap complete: %d books indexed", total)
	return nil
}
