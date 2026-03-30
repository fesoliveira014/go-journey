package handler_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
	"github.com/fesoliveira014/library-system/services/gateway/internal/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockAuthClient implements authv1.AuthServiceClient for testing.
type mockAuthClient struct {
	loginFn        func(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.AuthResponse, error)
	registerFn     func(ctx context.Context, in *authv1.RegisterRequest, opts ...grpc.CallOption) (*authv1.AuthResponse, error)
	initOAuth2Fn   func(ctx context.Context, in *authv1.InitOAuth2Request, opts ...grpc.CallOption) (*authv1.InitOAuth2Response, error)
	completeOAuth2Fn func(ctx context.Context, in *authv1.CompleteOAuth2Request, opts ...grpc.CallOption) (*authv1.AuthResponse, error)
}

func (m *mockAuthClient) Login(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.AuthResponse, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockAuthClient) Register(ctx context.Context, in *authv1.RegisterRequest, opts ...grpc.CallOption) (*authv1.AuthResponse, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockAuthClient) ValidateToken(ctx context.Context, in *authv1.ValidateTokenRequest, opts ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockAuthClient) GetUser(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.User, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockAuthClient) InitOAuth2(ctx context.Context, in *authv1.InitOAuth2Request, opts ...grpc.CallOption) (*authv1.InitOAuth2Response, error) {
	if m.initOAuth2Fn != nil {
		return m.initOAuth2Fn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockAuthClient) CompleteOAuth2(ctx context.Context, in *authv1.CompleteOAuth2Request, opts ...grpc.CallOption) (*authv1.AuthResponse, error) {
	if m.completeOAuth2Fn != nil {
		return m.completeOAuth2Fn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// authTestTemplates builds a template map sufficient for auth handler tests.
// The base.html named template outputs the content block directly so tests can
// inspect rendered form fields and error messages.
func authTestTemplates(t *testing.T, pages ...string) map[string]*template.Template {
	t.Helper()
	m := make(map[string]*template.Template)
	for _, page := range pages {
		set := template.Must(template.New("base.html").Parse(
			`{{template "content" .}}`,
		))
		template.Must(set.New("nav").Parse(`{{define "nav"}}{{end}}`))
		template.Must(set.New("flash").Parse(`{{define "flash"}}{{end}}`))
		// Default no-op content; the page template will override this.
		template.Must(set.New("content").Parse(`{{define "content"}}{{end}}`))
		m[page] = set
	}
	// Also include error.html so renderError works.
	errSet := template.Must(template.New("base.html").Parse(`ERROR:{{.Data.Status}}:{{.Data.Message}}`))
	template.Must(errSet.New("nav").Parse(`{{define "nav"}}{{end}}`))
	template.Must(errSet.New("flash").Parse(`{{define "flash"}}{{end}}`))
	template.Must(errSet.New("content").Parse(`{{define "content"}}{{end}}`))
	m["error.html"] = errSet
	return m
}

// loginTemplates returns a map with a login.html template that renders the
// Error field so tests can assert on it.
func loginTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()
	set := template.Must(template.New("base.html").Parse(
		`Login{{if .Data}}{{if .Data.Error}} ERROR:{{.Data.Error}}{{end}}{{end}}`,
	))
	errSet := template.Must(template.New("base.html").Parse(`ERROR:{{.Data.Status}}:{{.Data.Message}}`))
	return map[string]*template.Template{
		"login.html": set,
		"error.html": errSet,
	}
}

// registerTemplates returns a map with a register.html template.
func registerTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()
	set := template.Must(template.New("base.html").Parse(
		`Register{{if .Data}}{{if .Data.Error}} ERROR:{{.Data.Error}}{{end}}{{end}}`,
	))
	errSet := template.Must(template.New("base.html").Parse(`ERROR:{{.Data.Status}}:{{.Data.Message}}`))
	return map[string]*template.Template{
		"register.html": set,
		"error.html":    errSet,
	}
}

// ---- Tests ----

func TestLoginPage_RendersForm(t *testing.T) {
	tmpl := loginTemplates(t)
	srv := handler.New(nil, nil, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()

	srv.LoginPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Login") {
		t.Errorf("expected body to contain \"Login\", got %q", rec.Body.String())
	}
}

func TestLoginSubmit_Success(t *testing.T) {
	mock := &mockAuthClient{
		loginFn: func(_ context.Context, _ *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
			return &authv1.AuthResponse{Token: "tok-abc"}, nil
		},
	}
	tmpl := loginTemplates(t)
	srv := handler.New(mock, nil, tmpl)

	form := url.Values{"email": {"user@example.com"}, "password": {"secret"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.LoginSubmit(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/books" {
		t.Errorf("expected redirect to /books, got %q", loc)
	}

	// Verify session cookie was set.
	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}
	if sessionCookie.Value != "tok-abc" {
		t.Errorf("expected session token %q, got %q", "tok-abc", sessionCookie.Value)
	}
}

func TestLoginSubmit_InvalidCredentials(t *testing.T) {
	mock := &mockAuthClient{
		loginFn: func(_ context.Context, _ *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		},
	}
	tmpl := loginTemplates(t)
	srv := handler.New(mock, nil, tmpl)

	form := url.Values{"email": {"user@example.com"}, "password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.LoginSubmit(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (form re-render), got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid email or password") {
		t.Errorf("expected error message in body, got %q", rec.Body.String())
	}
}

func TestLoginSubmit_EmptyFields(t *testing.T) {
	tmpl := loginTemplates(t)
	srv := handler.New(nil, nil, tmpl)

	form := url.Values{"email": {""}, "password": {""}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.LoginSubmit(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (form re-render), got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Email and password are required") {
		t.Errorf("expected validation error in body, got %q", rec.Body.String())
	}
}

func TestRegisterSubmit_Success(t *testing.T) {
	mock := &mockAuthClient{
		registerFn: func(_ context.Context, _ *authv1.RegisterRequest, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
			return &authv1.AuthResponse{Token: "tok-new"}, nil
		},
	}
	tmpl := registerTemplates(t)
	srv := handler.New(mock, nil, tmpl)

	form := url.Values{
		"email":    {"new@example.com"},
		"password": {"pass123"},
		"name":     {"Alice"},
	}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.RegisterSubmit(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/books" {
		t.Errorf("expected redirect to /books, got %q", loc)
	}

	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}
	if sessionCookie.Value != "tok-new" {
		t.Errorf("expected session token %q, got %q", "tok-new", sessionCookie.Value)
	}
}

func TestLogout_ClearsCookie(t *testing.T) {
	srv := handler.New(nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "tok-abc"})
	rec := httptest.NewRecorder()

	srv.Logout(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %q", loc)
	}

	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie in response (clearing it)")
	}
	if sessionCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge=-1 to clear cookie, got %d", sessionCookie.MaxAge)
	}
}

func TestOAuth2Start_RedirectsToGoogle(t *testing.T) {
	const oauthURL = "https://accounts.google.com/o/oauth2/auth?client_id=test"
	mock := &mockAuthClient{
		initOAuth2Fn: func(_ context.Context, _ *authv1.InitOAuth2Request, _ ...grpc.CallOption) (*authv1.InitOAuth2Response, error) {
			return &authv1.InitOAuth2Response{RedirectUrl: oauthURL}, nil
		},
	}
	tmpl := authTestTemplates(t, "error.html")
	srv := handler.New(mock, nil, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/auth/oauth2/google", nil)
	rec := httptest.NewRecorder()

	srv.OAuth2Start(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != oauthURL {
		t.Errorf("expected redirect to %q, got %q", oauthURL, loc)
	}
}

func TestOAuth2Callback_Success(t *testing.T) {
	mock := &mockAuthClient{
		completeOAuth2Fn: func(_ context.Context, in *authv1.CompleteOAuth2Request, _ ...grpc.CallOption) (*authv1.AuthResponse, error) {
			if in.Code != "mycode" || in.State != "mystate" {
				return nil, status.Error(codes.InvalidArgument, "bad params")
			}
			return &authv1.AuthResponse{Token: "tok-oauth"}, nil
		},
	}
	tmpl := authTestTemplates(t, "error.html")
	srv := handler.New(mock, nil, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/auth/oauth2/google/callback?code=mycode&state=mystate", nil)
	rec := httptest.NewRecorder()

	srv.OAuth2Callback(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/books" {
		t.Errorf("expected redirect to /books, got %q", loc)
	}

	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}
	if sessionCookie.Value != "tok-oauth" {
		t.Errorf("expected session token %q, got %q", "tok-oauth", sessionCookie.Value)
	}
}
