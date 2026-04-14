package handler

import (
	"html/template"
	"os"
	"path/filepath"

	"github.com/gorilla/securecookie"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	catalogv1 "github.com/fesoliveira014/library-system/gen/catalog/v1"
	reservationv1 "github.com/fesoliveira014/library-system/gen/reservation/v1"
	searchv1 "github.com/fesoliveira014/library-system/gen/search/v1"
)

type Server struct {
	auth        authv1.AuthServiceClient
	catalog     catalogv1.CatalogServiceClient
	reservation reservationv1.ReservationServiceClient
	search      searchv1.SearchServiceClient
	tmpl        map[string]*template.Template
	baseTmpl    *template.Template // base set for rendering partials
	flash       *securecookie.SecureCookie
}

// Option configures optional fields on the Server. Used with New.
type Option func(*Server)

// WithFlashKey configures the HMAC key used to sign the flash cookie. In
// production this should be read from the FLASH_COOKIE_KEY environment
// variable and be at least 32 random bytes. When no key is supplied, New
// generates a random per-process key — fine for tests, but it means flashes
// do not survive a restart.
func WithFlashKey(hashKey []byte) Option {
	return func(s *Server) {
		s.flash = securecookie.New(hashKey, nil)
	}
}

func New(auth authv1.AuthServiceClient, catalog catalogv1.CatalogServiceClient, reservation reservationv1.ReservationServiceClient, search searchv1.SearchServiceClient, tmpl map[string]*template.Template, opts ...Option) *Server {
	// Pick any entry for partial rendering — all share the same partial definitions.
	var base *template.Template
	for _, t := range tmpl {
		base = t
		break
	}
	s := &Server{auth: auth, catalog: catalog, reservation: reservation, search: search, tmpl: tmpl, baseTmpl: base}
	for _, opt := range opts {
		opt(s)
	}
	if s.flash == nil {
		s.flash = securecookie.New(securecookie.GenerateRandomKey(32), nil)
	}
	return s
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
