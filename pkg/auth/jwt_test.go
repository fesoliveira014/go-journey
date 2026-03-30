package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	auth "github.com/fesoliveira014/library-system/pkg/auth"
)

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret-key"
	userID := uuid.New()
	role := "user"

	token, err := auth.GenerateToken(userID, role, secret, time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := auth.ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("expected user ID %s, got %s", userID, claims.UserID)
	}
	if claims.Role != role {
		t.Errorf("expected role %q, got %q", role, claims.Role)
	}
}

func TestValidateToken_InvalidSecret(t *testing.T) {
	userID := uuid.New()
	token, _ := auth.GenerateToken(userID, "user", "secret-1", time.Hour)

	_, err := auth.ValidateToken(token, "secret-2")
	if err == nil {
		t.Fatal("expected error for invalid secret")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	userID := uuid.New()
	token, _ := auth.GenerateToken(userID, "user", "secret", -time.Hour)

	_, err := auth.ValidateToken(token, "secret")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	_, err := auth.ValidateToken("not.a.jwt", "secret")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}
