package handler

import (
	"context"
	"log"
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
		Flash: consumeFlash(w, r),
		Data:  data,
	}
	tmpl, ok := s.tmpl[name]
	if !ok {
		log.Printf("template not found: %q", name)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if s.baseTmpl == nil {
		log.Printf("no templates loaded; cannot render partial %q", name)
		return
	}
	if err := s.baseTmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	pd := PageData{
		User:  userFromContext(r.Context()),
		Flash: consumeFlash(w, r),
		Data: map[string]any{
			"Status":  code,
			"Message": message,
		},
	}
	tmpl, ok := s.tmpl["error.html"]
	if !ok {
		log.Printf("error template not found")
		http.Error(w, message, code)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
		log.Printf("template error: %v", err)
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

func setFlash(w http.ResponseWriter, message string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flash",
		Value:    message,
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

func consumeFlash(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie("flash")
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "flash",
		Path:   "/",
		MaxAge: -1,
	})
	return c.Value
}
