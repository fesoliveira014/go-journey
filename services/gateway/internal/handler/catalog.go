package handler

import (
	"net/http"
	"strconv"

	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
)

func (s *Server) Home(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "home.html", map[string]any{})
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

// requireAdmin checks that the current user is authenticated and has the
// "admin" role. Returns true if the check passes; otherwise writes the
// appropriate response and returns false.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	u := userFromContext(r.Context())
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return false
	}
	if u.Role != "admin" {
		s.renderError(w, r, http.StatusForbidden, "Access denied")
		return false
	}
	return true
}

func (s *Server) AdminBookNew(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	s.render(w, r, "admin_book_new.html", map[string]any{})
}

func (s *Server) AdminBookCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}

	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	title := r.FormValue("title")
	author := r.FormValue("author")
	isbn := r.FormValue("isbn")
	genre := r.FormValue("genre")
	description := r.FormValue("description")
	publishedYearStr := r.FormValue("published_year")
	totalCopiesStr := r.FormValue("total_copies")

	if title == "" || author == "" || isbn == "" || genre == "" || publishedYearStr == "" || totalCopiesStr == "" {
		s.render(w, r, "admin_book_new.html", map[string]any{
			"Error": "Title, author, ISBN, genre, published year, and total copies are required",
		})
		return
	}

	publishedYear, err := strconv.Atoi(publishedYearStr)
	if err != nil {
		s.render(w, r, "admin_book_new.html", map[string]any{
			"Error": "Published year must be a number",
		})
		return
	}

	totalCopies, err := strconv.Atoi(totalCopiesStr)
	if err != nil {
		s.render(w, r, "admin_book_new.html", map[string]any{
			"Error": "Total copies must be a number",
		})
		return
	}

	_, err = s.catalog.CreateBook(r.Context(), &catalogv1.CreateBookRequest{
		Title:         title,
		Author:        author,
		Isbn:          isbn,
		Genre:         genre,
		Description:   description,
		PublishedYear: int32(publishedYear),
		TotalCopies:   int32(totalCopies),
	})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to create book")
		return
	}

	setFlash(w, "Book created")
	http.Redirect(w, r, "/books", http.StatusSeeOther)
}

func (s *Server) AdminBookEdit(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	book, err := s.catalog.GetBook(r.Context(), &catalogv1.GetBookRequest{Id: id})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to load book")
		return
	}
	s.render(w, r, "admin_book_edit.html", book)
}

func (s *Server) AdminBookUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}

	id := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	title := r.FormValue("title")
	author := r.FormValue("author")
	isbn := r.FormValue("isbn")
	genre := r.FormValue("genre")
	description := r.FormValue("description")
	publishedYearStr := r.FormValue("published_year")
	totalCopiesStr := r.FormValue("total_copies")

	formData := map[string]any{
		"Id": id, "Title": title, "Author": author, "Isbn": isbn,
		"Genre": genre, "Description": description,
		"PublishedYear": publishedYearStr, "TotalCopies": totalCopiesStr,
	}

	if title == "" || author == "" || isbn == "" || genre == "" || publishedYearStr == "" || totalCopiesStr == "" {
		formData["Error"] = "Title, author, ISBN, genre, published year, and total copies are required"
		s.render(w, r, "admin_book_edit.html", formData)
		return
	}

	publishedYear, err := strconv.Atoi(publishedYearStr)
	if err != nil {
		formData["Error"] = "Published year must be a number"
		s.render(w, r, "admin_book_edit.html", formData)
		return
	}

	totalCopies, err := strconv.Atoi(totalCopiesStr)
	if err != nil {
		formData["Error"] = "Total copies must be a number"
		s.render(w, r, "admin_book_edit.html", formData)
		return
	}

	_, err = s.catalog.UpdateBook(r.Context(), &catalogv1.UpdateBookRequest{
		Id:            id,
		Title:         title,
		Author:        author,
		Isbn:          isbn,
		Genre:         genre,
		Description:   description,
		PublishedYear: int32(publishedYear),
		TotalCopies:   int32(totalCopies),
	})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to update book")
		return
	}

	setFlash(w, "Book updated")
	http.Redirect(w, r, "/books/"+id, http.StatusSeeOther)
}

func (s *Server) AdminBookDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	_, err := s.catalog.DeleteBook(r.Context(), &catalogv1.DeleteBookRequest{Id: id})
	if err != nil {
		s.handleGRPCError(w, r, err, "Failed to delete book")
		return
	}
	setFlash(w, "Book deleted")
	http.Redirect(w, r, "/books", http.StatusSeeOther)
}
