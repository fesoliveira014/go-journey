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

// ExportedConsumeFlash calls the unexported consumeFlash method.
func (s *Server) ExportedConsumeFlash(w http.ResponseWriter, r *http.Request) string {
	return s.consumeFlash(w, r)
}

// ExportedSetFlash calls the unexported setFlash method.
func (s *Server) ExportedSetFlash(w http.ResponseWriter, message string) {
	s.setFlash(w, message)
}

// DecodeFlashFromResponse is a test helper that finds the flash cookie on the
// recorded response and decodes its signed value back to the original message.
// Returns the empty string if no flash cookie was set or verification fails.
func (s *Server) DecodeFlashFromResponse(cookies []*http.Cookie) string {
	for _, c := range cookies {
		if c.Name != "flash" || c.Value == "" {
			continue
		}
		var message string
		if err := s.flash.Decode("flash", c.Value, &message); err != nil {
			return ""
		}
		return message
	}
	return ""
}
