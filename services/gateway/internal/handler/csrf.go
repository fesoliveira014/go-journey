package handler

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
)

const csrfCookieName = "csrf_token"

// CSRF rejects unsafe requests whose submitted token does not match the
// signed CSRF cookie. Safe methods pass through unchanged.
func (s *Server) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			next.ServeHTTP(w, r)
			return
		}

		if !s.validCSRF(r) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) csrfToken(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookieName); err == nil {
		var token string
		if err := s.csrf.Decode(csrfCookieName, c.Value, &token); err == nil && token != "" {
			return token
		}
	}

	token, err := randomToken()
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to generate csrf token", "error", err)
		return ""
	}
	encoded, err := s.csrf.Encode(csrfCookieName, token)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to sign csrf token", "error", err)
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies,
	})
	return token
}

func (s *Server) validCSRF(r *http.Request) bool {
	c, err := r.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	var cookieToken string
	if err := s.csrf.Decode(csrfCookieName, c.Value, &cookieToken); err != nil || cookieToken == "" {
		return false
	}
	submitted := r.FormValue("_csrf")
	if submitted == "" {
		submitted = r.Header.Get("X-CSRF-Token")
	}
	if submitted == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(submitted), []byte(cookieToken)) == 1
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
