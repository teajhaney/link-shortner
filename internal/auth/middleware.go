package auth

import (
	"context"
	"errors"
	"link-shortner/internal/response"
	"net/http"
	"strings"
)

type contextKey string

const userIDKey contextKey = "userID"

// extractToken pulls the bearer token out of the Authorization header. It
// returns "" when the header is missing or not in "Bearer <token>" form.
func extractToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// writeAuthError maps the token sentinel errors onto HTTP status codes. An
// expired, invalid, or revoked token is all a 401; the message names which one
// so a client can tell "time passed" from "you signed out".
func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrExpiredToken),
		errors.Is(err, ErrInvalidToken),
		errors.Is(err, ErrTokenRevoked):
		response.WriteError(w, http.StatusUnauthorized, err.Error())
	default:
		response.WriteError(w, http.StatusInternalServerError, "Failed to verify token")
	}
}

// TokenVerifier verifies an access token and returns its claims. Both
// jwtService, which checks only the signature and expiry, and TokenValidator,
// which also consults the revocation list, satisfy it.
type TokenVerifier interface {
	Verify(tokenString string) (*Claims, error)
}

// Middleware returns middleware that requires a valid bearer token. On success
// it puts the authenticated user ID in the request context, where downstream
// handlers read it with UserIDFromContext.
//
// Pass a TokenValidator here: handing it the plain JWT service would accept a
// token that logout has already revoked.
func Middleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				response.WriteError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
				return
			}

			claims, err := verifier.Verify(token)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			// Storing the user ID in context means handlers do not each parse
			// the token again; the middleware is the single authority on who
			// the caller is.
			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext returns the authenticated user ID, or "" when the request
// did not pass through the auth middleware.
func UserIDFromContext(ctx context.Context) string {
	userID, _ := ctx.Value(userIDKey).(string)
	return userID
}
