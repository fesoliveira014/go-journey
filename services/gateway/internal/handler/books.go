package handler

import (
	"encoding/json"
	"net/http"
)

// Book represents a book in the library catalog.
// In later chapters, this will be replaced by the Catalog service's protobuf type.
type Book struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Genre  string `json:"genre"`
	Year   int    `json:"year"`
}

// sampleBooks is a hardcoded list for Chapter 1. It will be replaced
// by gRPC calls to the Catalog service in Chapter 5.
var sampleBooks = []Book{
	{ID: "1", Title: "The Go Programming Language", Author: "Alan Donovan & Brian Kernighan", Genre: "Programming", Year: 2015},
	{ID: "2", Title: "Designing Data-Intensive Applications", Author: "Martin Kleppmann", Genre: "Distributed Systems", Year: 2017},
	{ID: "3", Title: "Building Microservices", Author: "Sam Newman", Genre: "Architecture", Year: 2021},
}

func Books(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sampleBooks)
}
