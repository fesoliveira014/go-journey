package handler

import (
	"net/http"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
)

// LoginPage renders the login form.
func (s *Server) LoginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login.html", map[string]any{})
}

// LoginSubmit handles POST /login.
func (s *Server) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	if email == "" || password == "" {
		s.render(w, r, "login.html", map[string]any{"Error": "Email and password are required", "Email": email})
		return
	}
	resp, err := s.auth.Login(r.Context(), &authv1.LoginRequest{Email: email, Password: password})
	if err != nil {
		s.render(w, r, "login.html", map[string]any{"Error": "Invalid email or password", "Email": email})
		return
	}
	setSessionCookie(w, resp.Token)
	setFlash(w, "Welcome back!")
	http.Redirect(w, r, "/books", http.StatusSeeOther)
}

// RegisterPage renders the registration form.
func (s *Server) RegisterPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "register.html", map[string]any{})
}

// RegisterSubmit handles POST /register.
func (s *Server) RegisterSubmit(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	name := r.FormValue("name")
	if email == "" || password == "" || name == "" {
		s.render(w, r, "register.html", map[string]any{
			"Error": "Email, password, and name are required",
			"Email": email,
			"Name":  name,
		})
		return
	}
	resp, err := s.auth.Register(r.Context(), &authv1.RegisterRequest{
		Email:    email,
		Password: password,
		Name:     name,
	})
	if err != nil {
		s.render(w, r, "register.html", map[string]any{
			"Error": "Registration failed. Please try again.",
			"Email": email,
			"Name":  name,
		})
		return
	}
	setSessionCookie(w, resp.Token)
	setFlash(w, "Welcome! Your account has been created.")
	http.Redirect(w, r, "/books", http.StatusSeeOther)
}

// Logout handles POST /logout.
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// OAuth2Start handles GET /auth/oauth2/google.
func (s *Server) OAuth2Start(w http.ResponseWriter, r *http.Request) {
	resp, err := s.auth.InitOAuth2(r.Context(), &authv1.InitOAuth2Request{})
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Failed to initiate OAuth2 login")
		return
	}
	http.Redirect(w, r, resp.RedirectUrl, http.StatusFound)
}

// OAuth2Callback handles GET /auth/oauth2/google/callback.
func (s *Server) OAuth2Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	resp, err := s.auth.CompleteOAuth2(r.Context(), &authv1.CompleteOAuth2Request{
		Code:  code,
		State: state,
	})
	if err != nil {
		s.renderError(w, r, http.StatusUnauthorized, "OAuth2 login failed")
		return
	}
	setSessionCookie(w, resp.Token)
	setFlash(w, "Welcome!")
	http.Redirect(w, r, "/books", http.StatusSeeOther)
}

// setSessionCookie writes a persistent session cookie to the response.
func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
}

// clearSessionCookie removes the session cookie from the browser.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Path:   "/",
		MaxAge: -1,
	})
}
