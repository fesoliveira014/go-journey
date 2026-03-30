package handler

import (
	"net/http"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
)

func (s *Server) Home(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "home.html", nil)
}

func (s *Server) BookList(w http.ResponseWriter, r *http.Request) {
	genre := r.URL.Query().Get("genre")
	resp, err := s.catalog.ListBooks(r.Context(), &catalogv1.ListBooksRequest{
		Genre: genre,
	})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to load books")
		return
	}
	if r.Header.Get("HX-Request") != "" {
		s.renderPartial(w, "book_cards", resp.Books)
		return
	}
	s.render(w, r, "catalog.html", map[string]any{
		"Books":         resp.Books,
		"SelectedGenre": genre,
	})
}

func (s *Server) BookDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	book, err := s.catalog.GetBook(r.Context(), &catalogv1.GetBookRequest{Id: id})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to load book")
		return
	}
	s.render(w, r, "book.html", book)
}
