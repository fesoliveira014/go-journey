package index

import (
	"context"
	"fmt"
	"strings"

	"github.com/meilisearch/meilisearch-go"

	"github.com/fesoliveira014/library-system/services/search/internal/model"
)

const indexName = "books"

// SearchFilters holds optional filter parameters for search queries.
type SearchFilters struct {
	Genre         string
	Author        string
	AvailableOnly bool
}

// IndexRepository defines the interface for the search index data store.
type IndexRepository interface {
	Upsert(ctx context.Context, doc model.BookDocument) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, query string, filters SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error)
	Suggest(ctx context.Context, prefix string, limit int) ([]model.BookDocument, error)
	Count(ctx context.Context) (int64, error)
	EnsureIndex(ctx context.Context) error
}

// MeilisearchIndex implements IndexRepository backed by Meilisearch.
type MeilisearchIndex struct {
	client meilisearch.ServiceManager
}

// NewMeilisearchIndex creates a new Meilisearch-backed index.
func NewMeilisearchIndex(url, apiKey string) *MeilisearchIndex {
	client := meilisearch.New(url, meilisearch.WithAPIKey(apiKey))
	return &MeilisearchIndex{client: client}
}

// EnsureIndex creates the "books" index (if it doesn't exist) and configures
// searchable, filterable, and sortable attributes. Meilisearch operations are
// asynchronous — they enqueue tasks and return immediately. Newly indexed
// documents may not be searchable until the task completes (usually <1s).
func (m *MeilisearchIndex) EnsureIndex(_ context.Context) error {
	_, err := m.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        indexName,
		PrimaryKey: "id",
	})
	// Ignore "index_already_exists" error
	if err != nil {
		if meiliErr, ok := err.(*meilisearch.Error); ok {
			if meiliErr.MeilisearchApiError.Code == "index_already_exists" {
				// Index exists, continue to configure attributes
			} else {
				return fmt.Errorf("create index: %w", err)
			}
		} else {
			return fmt.Errorf("create index: %w", err)
		}
	}

	idx := m.client.Index(indexName)

	if _, err := idx.UpdateSearchableAttributes(&[]string{
		"title", "author", "isbn", "description", "genre",
	}); err != nil {
		return fmt.Errorf("update searchable attributes: %w", err)
	}
	if _, err := idx.UpdateFilterableAttributes(&[]interface{}{
		"genre", "author", "available_copies",
	}); err != nil {
		return fmt.Errorf("update filterable attributes: %w", err)
	}
	if _, err := idx.UpdateSortableAttributes(&[]string{
		"title", "published_year",
	}); err != nil {
		return fmt.Errorf("update sortable attributes: %w", err)
	}

	return nil
}

func (m *MeilisearchIndex) Upsert(_ context.Context, doc model.BookDocument) error {
	idx := m.client.Index(indexName)
	pk := "id"
	_, err := idx.AddDocuments([]model.BookDocument{doc}, &meilisearch.DocumentOptions{PrimaryKey: &pk})
	if err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}
	return nil
}

func (m *MeilisearchIndex) Delete(_ context.Context, id string) error {
	idx := m.client.Index(indexName)
	_, err := idx.DeleteDocument(id, nil)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}

func (m *MeilisearchIndex) Search(_ context.Context, query string, filters SearchFilters, page, pageSize int) ([]model.BookDocument, int64, int64, error) {
	idx := m.client.Index(indexName)

	filterParts := buildFilterString(filters)

	offset := int64(0)
	if page > 1 {
		offset = int64((page - 1) * pageSize)
	}

	req := &meilisearch.SearchRequest{
		Limit:  int64(pageSize),
		Offset: offset,
	}
	if len(filterParts) > 0 {
		req.Filter = strings.Join(filterParts, " AND ")
	}

	resp, err := idx.Search(query, req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("search: %w", err)
	}

	docs := make([]model.BookDocument, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		doc, err := hitToDocument(hit)
		if err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	return docs, resp.EstimatedTotalHits, int64(resp.ProcessingTimeMs), nil
}

func (m *MeilisearchIndex) Suggest(_ context.Context, prefix string, limit int) ([]model.BookDocument, error) {
	idx := m.client.Index(indexName)

	resp, err := idx.Search(prefix, &meilisearch.SearchRequest{
		Limit:                int64(limit),
		AttributesToRetrieve: []string{"id", "title", "author"},
	})
	if err != nil {
		return nil, fmt.Errorf("suggest: %w", err)
	}

	docs := make([]model.BookDocument, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		doc, err := hitToDocument(hit)
		if err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

func (m *MeilisearchIndex) Count(_ context.Context) (int64, error) {
	stats, err := m.client.Index(indexName).GetStats()
	if err != nil {
		return 0, fmt.Errorf("get index stats: %w", err)
	}
	return stats.NumberOfDocuments, nil
}

func buildFilterString(filters SearchFilters) []string {
	var parts []string
	if filters.Genre != "" {
		parts = append(parts, fmt.Sprintf("genre = %q", filters.Genre))
	}
	if filters.Author != "" {
		parts = append(parts, fmt.Sprintf("author = %q", filters.Author))
	}
	if filters.AvailableOnly {
		parts = append(parts, "available_copies > 0")
	}
	return parts
}

func hitToDocument(hit interface{}) (model.BookDocument, error) {
	m, ok := hit.(map[string]interface{})
	if !ok {
		return model.BookDocument{}, fmt.Errorf("unexpected hit type")
	}

	doc := model.BookDocument{}
	if v, ok := m["id"].(string); ok {
		doc.ID = v
	}
	if v, ok := m["title"].(string); ok {
		doc.Title = v
	}
	if v, ok := m["author"].(string); ok {
		doc.Author = v
	}
	if v, ok := m["isbn"].(string); ok {
		doc.ISBN = v
	}
	if v, ok := m["genre"].(string); ok {
		doc.Genre = v
	}
	if v, ok := m["description"].(string); ok {
		doc.Description = v
	}
	if v, ok := m["published_year"].(float64); ok {
		doc.PublishedYear = int(v)
	}
	if v, ok := m["total_copies"].(float64); ok {
		doc.TotalCopies = int(v)
	}
	if v, ok := m["available_copies"].(float64); ok {
		doc.AvailableCopies = int(v)
	}
	return doc, nil
}
