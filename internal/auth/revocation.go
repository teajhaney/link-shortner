package auth

import (
	"link-shortner/internal/database"
	"time"
)

// TokenValidator verifies a token and consults the revocation list. It exists
// because three call sites need the same two checks in the same order: the
// auth middleware, the logout handler, and the verify endpoint. Doing the
// revocation lookup here keeps the JWT service focused on signing.
type TokenValidator struct {
	tokens  *jwtService
	revoked database.RevokedTokens
}

// NewTokenValidator wires a JWT service to a revocation store. The store must
// be the same Postgres pool the rest of the app uses, so a token revoked by
// any instance is rejected by all of them.
func NewTokenValidator(tokens *jwtService, revoked database.RevokedTokens) *TokenValidator {
	return &TokenValidator{tokens: tokens, revoked: revoked}
}

// Verify checks the signature and expiry, then rejects tokens that have been
// revoked. Order matters: the signature is checked before a database lookup is
// spent on it, so an unsigned or tampered token never touches Postgres.
func (v *TokenValidator) Verify(tokenString string) (*Claims, error) {
	claims, err := v.tokens.Verify(tokenString)
	if err != nil {
		return nil, err
	}

	revoked, err := v.revoked.IsRevoked(claims.ID)
	if err != nil {
		return nil, err
	}
	if revoked {
		return nil, ErrTokenRevoked
	}

	return claims, nil
}

// Revoke records a token as invalid. It is called on logout.
func (v *TokenValidator) Revoke(claims *Claims) error {
	return v.revoked.Revoke(claims.ID, claims.UserID, time.Now())
}
