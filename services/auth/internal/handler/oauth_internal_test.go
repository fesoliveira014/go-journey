package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/fesoliveira014/library-system/gen/auth/v1"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCompleteOAuth2_ExchangeFailureReturnsGenericError(t *testing.T) {
	t.Parallel()

	const tokenURL = "https://oauth.example.test/token"
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != tokenURL {
				t.Fatalf("unexpected token URL %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     "500 Internal Server Error",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("provider detail should stay server-side")),
				Request:    req,
			}, nil
		}),
	}

	h := NewAuthHandler(nil)
	h.oauthConfig = &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost:8080/auth/oauth2/google/callback",
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenURL,
		},
	}
	h.states["valid-state"] = time.Now().Add(time.Minute)

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)
	_, err := h.CompleteOAuth2(ctx, &authv1.CompleteOAuth2Request{
		Code:  "bad-code",
		State: "valid-state",
	})
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", st.Code())
	}
	if st.Message() != "authentication failed" {
		t.Fatalf("expected generic error message, got %q", st.Message())
	}
	if strings.Contains(st.Message(), "provider detail") || strings.Contains(st.Message(), tokenURL) {
		t.Fatalf("OAuth provider details leaked to client: %q", st.Message())
	}
}
