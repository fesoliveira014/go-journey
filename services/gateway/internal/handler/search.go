package handler

import (
	"net/http"
	"strconv"

	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
)

func (s *Server) SearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	genre := r.URL.Query().Get("genre")
	author := r.URL.Query().Get("author")
	available := r.URL.Query().Get("available")
	pageStr := r.URL.Query().Get("page")

	page, _ := strconv.Atoi(pageStr)
	if page <= 0 {
		page = 1
	}

	data := map[string]any{
		"Query":     query,
		"Genre":     genre,
		"Author":    author,
		"Available": available == "on",
		"Page":      page,
	}

	if query == "" {
		s.render(w, r, "search.html", data)
		return
	}

	resp, err := s.search.Search(r.Context(), &searchv1.SearchRequest{
		Query:         query,
		Genre:         genre,
		Author:        author,
		AvailableOnly: available == "on",
		Page:          int32(page),
		PageSize:      20,
	})
	if err != nil {
		s.handleGRPCError(w, r, err, "Search failed")
		return
	}

	data["Books"] = resp.Books
	data["TotalHits"] = resp.TotalHits
	data["QueryTimeMs"] = resp.QueryTimeMs
	data["HasResults"] = len(resp.Books) > 0

	s.render(w, r, "search.html", data)
}

func (s *Server) SearchSuggest(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	if len(prefix) < 2 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}

	resp, err := s.search.Suggest(r.Context(), &searchv1.SuggestRequest{
		Prefix: prefix,
		Limit:  5,
	})
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}

	s.renderPartial(w, "suggestions.html", resp.Suggestions)
}
