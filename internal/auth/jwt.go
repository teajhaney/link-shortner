package auth

import (
	"errors"
	"fmt"
	"time"

	"link-shortner/internal/shortcode"

	"github.com/golang-jwt/jwt/v5"
)

// Errors returned by the JWT service. Callers map these to HTTP status codes:
// ErrInvalidToken and ErrExpiredToken are 401s, anything else is a 500.
var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
	// ErrTokenRevoked is returned for a token whose signature and expiry are
	// still valid but which logout has explicitly invalidated. It is a 401,
	// distinct from ErrExpiredToken so a client can tell "sign in again
	// because you logged out" from "sign in again because time passed".
	ErrTokenRevoked = errors.New("token has been revoked")
)

// tokenLifetime is how long an access token stays valid after it is issued.
// It is deliberately short: an access token cannot be revoked before it
// expires without a database lookup on every request, so the cheap way to
// limit the damage of a leaked token is to make it expire quickly. Staying
// signed in is the refresh token's job, not this one's.
const tokenLifetime = 15 * time.Minute

// jwtService issues and verifies access tokens. It is safe for concurrent use
// because it holds no mutable state after New.
type jwtService struct {
	secret []byte
	issuer string
}

// NewJWTService builds a JWT service from a signing secret. The secret must be
// at least 32 bytes: HS256 is only as strong as the key, and a short or empty
// key would let anyone forge tokens.
func NewJWTService(secret string, issuer string) (*jwtService, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("jwt secret must be at least 32 bytes, got %d", len(secret))
	}
	return &jwtService{secret: []byte(secret), issuer: issuer}, nil
}

// Claims is what the token carries. Registered claims (sub, exp, iat) are
// standard JWT fields; UserID is the custom claim that identifies the user.
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// Issue creates a signed token for the given user ID. The token expires
// tokenLifetime after now. It returns the compact, URL-safe string that
// clients send back in the Authorization header.
func (j *jwtService) Issue(userID string) (string, error) {
	now := time.Now()

	// The token ID ("jti") is what logout revokes. Without it there is nothing
	// unique to store in the revocation list, so this must be generated per
	// token, not derived from the user.
	tokenID, err := shortcode.RandomBase62(32)
	if err != nil {
		return "", err
	}

	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID: tokenID,
			// "sub" identifies the subject of the token. Using the user ID here
			// makes the token self-describing without a database lookup.
			Subject: userID,
			// "iss" identifies who issued it. Verifying it on the way back in
			// stops a token issued by another system sharing this secret from
			// being accepted.
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenLifetime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

// Verify validates a token string and returns its claims. It checks the
// signature, the expiry, and that the issuer matches. A token that fails any
// of those gets a sentinel error the caller can map to a 401.
func (j *jwtService) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}

	// jwt.Parse fills claims if the signature is valid, then returns the error
	// for anything wrong. The keyfunc asserts the signing method is HS256, so
	// a token signed with a different algorithm is rejected instead of being
	// verified against this key.
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	},
		// The issuer claim is what binds a token to this service: without
		// this check, a token minted by another system that happens to share
		// the secret would verify. The method list is belt and braces next to
		// the keyfunc assertion above, and requiring exp means a token can
		// never be accepted without an end date.
		jwt.WithIssuer(j.issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		// Map the library's errors to our own so handlers do not import the
		// JWT library to know what went wrong.
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
