package model

// BookDocument is the shape stored in and retrieved from Meilisearch.
type BookDocument struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Author          string `json:"author"`
	ISBN            string `json:"isbn"`
	Genre           string `json:"genre"`
	Description     string `json:"description"`
	PublishedYear   int    `json:"published_year"`
	TotalCopies     int    `json:"total_copies"`
	AvailableCopies int    `json:"available_copies"`
}

// Suggestion is a lightweight result for autocomplete.
type Suggestion struct {
	BookID string
	Title  string
	Author string
}
