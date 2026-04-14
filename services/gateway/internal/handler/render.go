package handler

import (
	"context"
	"log/slog"
	"net/http"

	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserInfo struct {
	ID   string
	Role string
}

type PageData struct {
	User  *UserInfo
	Flash string
	Data  any
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	pd := PageData{
		User:  userFromContext(r.Context()),
		Flash: s.consumeFlash(w, r),
		Data:  data,
	}
	tmpl, ok := s.tmpl[name]
	if !ok {
		slog.ErrorContext(r.Context(), "template not found", "name", name)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
		slog.ErrorContext(r.Context(), "template error", "error", err)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if s.baseTmpl == nil {
		slog.Error("no templates loaded", "partial", name)
		return
	}
	if err := s.baseTmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("template error", "error", err)
	}
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	pd := PageData{
		User:  userFromContext(r.Context()),
		Flash: s.consumeFlash(w, r),
		Data: map[string]any{
			"Status":  code,
			"Message": message,
		},
	}
	tmpl, ok := s.tmpl["error.html"]
	if !ok {
		slog.ErrorContext(r.Context(), "error template not found")
		http.Error(w, message, code)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
		slog.ErrorContext(r.Context(), "template error", "error", err)
	}
}

// userFromContext extracts UserInfo from the request context.
// Returns nil if no user is set (anonymous request).
func userFromContext(ctx context.Context) *UserInfo {
	uid, err := pkgauth.UserIDFromContext(ctx)
	if err != nil {
		return nil
	}
	role, _ := pkgauth.RoleFromContext(ctx)
	return &UserInfo{ID: uid.String(), Role: role}
}

// setFlash writes a short-lived flash cookie with the given message. The
// message is HMAC-signed via gorilla/securecookie so that a client cannot
// tamper with it: consumeFlash verifies the signature on read and discards
// the cookie if it is invalid. html/template already escapes the value on
// render, so signing is defence in depth, not the only guard.
func (s *Server) setFlash(w http.ResponseWriter, message string) {
	encoded, err := s.flash.Encode("flash", message)
	if err != nil {
		slog.Error("failed to encode flash cookie", "error", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "flash",
		Value:    encoded,
		Path:     "/",
		MaxAge:   10,
		HttpOnly: true,
	})
}

func (s *Server) handleGRPCError(w http.ResponseWriter, r *http.Request, err error, fallbackMsg string) {
	st, ok := status.FromError(err)
	if !ok {
		s.renderError(w, r, http.StatusInternalServerError, fallbackMsg)
		return
	}
	switch st.Code() {
	case codes.NotFound:
		s.renderError(w, r, http.StatusNotFound, "Not found")
	case codes.InvalidArgument:
		s.renderError(w, r, http.StatusBadRequest, st.Message())
	case codes.AlreadyExists:
		s.renderError(w, r, http.StatusConflict, st.Message())
	case codes.Unauthenticated:
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	case codes.PermissionDenied:
		s.renderError(w, r, http.StatusForbidden, "Access denied")
	case codes.ResourceExhausted:
		s.renderError(w, r, http.StatusTooManyRequests, "You have reached the maximum number of active reservations")
	case codes.FailedPrecondition:
		s.renderError(w, r, http.StatusPreconditionFailed, st.Message())
	default:
		s.renderError(w, r, http.StatusInternalServerError, fallbackMsg)
	}
}

// consumeFlash reads and clears the flash cookie. A cookie whose signature
// fails to verify (tampered, signed with a rotated key, or truncated) is
// treated as absent — the caller gets the empty string and the cookie is
// still cleared, so a poisoned cookie cannot wedge a user session.
func (s *Server) consumeFlash(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie("flash")
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "flash",
		Path:   "/",
		MaxAge: -1,
	})
	var message string
	if err := s.flash.Decode("flash", c.Value, &message); err != nil {
		return ""
	}
	return message
}
