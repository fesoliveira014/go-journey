// export_test.go exposes unexported symbols for use in external test packages.
// This file is compiled only during testing.
package handler

import "net/http"

// ExportedRender calls the unexported render method.
func (s *Server) ExportedRender(w http.ResponseWriter, r *http.Request, name string, data any) {
	s.render(w, r, name, data)
}

// ExportedRenderError calls the unexported renderError method.
func (s *Server) ExportedRenderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	s.renderError(w, r, code, message)
}

// ExportedConsumeFlash calls the unexported consumeFlash function.
func ExportedConsumeFlash(w http.ResponseWriter, r *http.Request) string {
	return consumeFlash(w, r)
}
