package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	auth "github.com/fesoliveira014/library-system/pkg/auth"
)

func TestGenerateAndValidateToken(t *testing.T) {
	t.Parallel()
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

func TestGenerateToken_UsesStableJSONClaimNames(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	tokenString, err := auth.GenerateToken(userID, "admin", "test-secret", time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	token, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("expected jwt.MapClaims, got %T", token.Claims)
	}

	if claims["user_id"] != userID.String() {
		t.Fatalf("expected user_id claim %q, got %v", userID.String(), claims["user_id"])
	}
	if claims["role"] != "admin" {
		t.Fatalf("expected role claim %q, got %v", "admin", claims["role"])
	}
	if _, ok := claims["UserID"]; ok {
		t.Fatal("token should not expose exported Go field name UserID")
	}
	if _, ok := claims["Role"]; ok {
		t.Fatal("token should not expose exported Go field name Role")
	}
}

func TestValidateToken_InvalidSecret(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	token, _ := auth.GenerateToken(userID, "user", "secret-1", time.Hour)

	_, err := auth.ValidateToken(token, "secret-2")
	if err == nil {
		t.Fatal("expected error for invalid secret")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	token, _ := auth.GenerateToken(userID, "user", "secret", -time.Hour)

	_, err := auth.ValidateToken(token, "secret")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	t.Parallel()
	_, err := auth.ValidateToken("not.a.jwt", "secret")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}
