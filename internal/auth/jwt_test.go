package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// newTestTokenService builds a service with a fixed 32-byte key so tests are
// deterministic and do not depend on the environment.
func newTestTokenService(t *testing.T) *jwtService {
	t.Helper()

	service, err := NewJWTService("01234567890123456789012345678901", "test-issuer")
	if err != nil {
		t.Fatalf("NewJWTService() error = %v", err)
	}
	return service
}

func TestNewJWTServiceRejectsShortSecret(t *testing.T) {
	if _, err := NewJWTService("short", "test-issuer"); err == nil {
		t.Fatal("NewJWTService() with a short secret should return an error")
	}
}

func TestIssueAndVerifyRoundTrip(t *testing.T) {
	service := newTestTokenService(t)

	token, err := service.Issue("550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if token == "" {
		t.Fatal("Issue() returned an empty token")
	}

	claims, err := service.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.UserID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("UserID = %q, want the issued user ID", claims.UserID)
	}
	if claims.Issuer != "test-issuer" {
		t.Fatalf("Issuer = %q, want %q", claims.Issuer, "test-issuer")
	}
	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt is not set")
	}
	// The token must outlive now by roughly the full lifetime, not be already
	// expired or valid for far longer than configured.
	wantExpiry := time.Now().Add(tokenLifetime)
	gotExpiry := claims.ExpiresAt.Time
	if diff := gotExpiry.Sub(wantExpiry); diff > time.Second || diff < -time.Second {
		t.Fatalf("ExpiresAt = %v, want ~%v", gotExpiry, wantExpiry)
	}
}

// A token signed with a different key verifies as invalid, not as valid with
// the wrong identity. This is the core security property of the service.
func TestVerifyRejectsTokenFromDifferentSecret(t *testing.T) {
	other, err := NewJWTService("99999999999999999999999999999999", "test-issuer")
	if err != nil {
		t.Fatalf("NewJWTService() error = %v", err)
	}

	token, err := other.Issue("attacker-user-id")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := newTestTokenService(t).Verify(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() error = %v, want ErrInvalidToken", err)
	}
}

// The issuer claim binds a token to this service. A token issued by a
// different system, even with the same key, must not be accepted.
func TestVerifyRejectsTokenFromDifferentIssuer(t *testing.T) {
	other, err := NewJWTService("01234567890123456789012345678901", "other-issuer")
	if err != nil {
		// The shared helper asserts the length, so a failure here is a test
		// bug rather than a product bug.
		t.Fatalf("NewJWTService() error = %v", err)
	}

	token, err := other.Issue("550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := newTestTokenService(t).Verify(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() error = %v, want ErrInvalidToken", err)
	}
}

func TestVerifyRejectsMalformedToken(t *testing.T) {
	if _, err := newTestTokenService(t).Verify("not.a.token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() error = %v, want ErrInvalidToken", err)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	// Build a token that already expired, then confirm Verify classifies it as
	// expired rather than merely invalid. The distinction matters because an
	// expired token means "sign in again", not "you are not allowed".
	service := newTestTokenService(t)

	claims := Claims{
		UserID: "550e8400-e29b-41d4-a716-446655440000",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "550e8400-e29b-41d4-a716-446655440000",
			Issuer:    "test-issuer",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(service.secret)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}

	if _, err := service.Verify(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("Verify() error = %v, want ErrExpiredToken", err)
	}
}
