package handler

import (
	"html/template"
	"os"
	"path/filepath"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
)

type Server struct {
	auth    authv1.AuthServiceClient
	catalog catalogv1.CatalogServiceClient
	tmpl    map[string]*template.Template
	baseTmpl *template.Template // base set for rendering partials
}

func New(auth authv1.AuthServiceClient, catalog catalogv1.CatalogServiceClient, tmpl map[string]*template.Template) *Server {
	// Pick any entry for partial rendering — all share the same partial definitions.
	var base *template.Template
	for _, t := range tmpl {
		base = t
		break
	}
	return &Server{auth: auth, catalog: catalog, tmpl: tmpl, baseTmpl: base}
}

// ParseTemplates builds a map of page name → cloned template set.
//
// The approach:
//  1. Parse base.html + all partials into a "base" set.
//  2. For each non-partial .html page file, clone the base set and parse
//     the page file into the clone so its {{define}} blocks override the
//     base {{block}} placeholders.
//  3. Return the map so each page gets an isolated template.Template that
//     can be executed by name "base.html" without the last-define-wins
//     problem that would arise from parsing all pages into a single set.
func ParseTemplates(templateDir string) (map[string]*template.Template, error) {
	baseFile := filepath.Join(templateDir, "base.html")
	partialsGlob := filepath.Join(templateDir, "partials", "*.html")

	partials, err := filepath.Glob(partialsGlob)
	if err != nil {
		return nil, err
	}

	// Build the list of files that make up the base set.
	baseFiles := append([]string{baseFile}, partials...)

	// Parse the base set once; we'll clone it for each page.
	baseSet, err := template.ParseFiles(baseFiles...)
	if err != nil {
		return nil, err
	}

	// Find all page templates (direct children of templateDir, not partials).
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, err
	}

	m := make(map[string]*template.Template)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".html" {
			continue
		}
		name := e.Name() // e.g. "home.html"
		if name == "base.html" {
			continue
		}
		pageFile := filepath.Join(templateDir, name)

		// Clone the base set and parse the page file into it.
		clone, err := baseSet.Clone()
		if err != nil {
			return nil, err
		}
		if _, err = clone.ParseFiles(pageFile); err != nil {
			return nil, err
		}
		m[name] = clone
	}

	return m, nil
}
