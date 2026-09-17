package auth

import (
	"errors"

	"link-shortner/internal/database"
)

// SigninService authenticates a user and starts a session. It owns the
// credential check; the JWT service owns access-token signing and the refresh
// service owns the long-lived half. Splitting them means each piece can be
// reused, and the token code never has to know about passwords.
type SigninService struct {
	users   database.Users
	tokens  *jwtService
	refresh *RefreshService
}

// NewSigninService wires the user store, the JWT service, and the refresh
// service together. The store must be the same one the users package uses, so
// the password hash written on signup is the one compared here.
func NewSigninService(users database.Users, tokens *jwtService, refresh *RefreshService) *SigninService {
	return &SigninService{users: users, tokens: tokens, refresh: refresh}
}

// Signin authenticates by email and password and returns a session: a short
// lived access token plus the refresh token that renews it.
//
// It deliberately returns the same error for a missing email and a wrong
// password: telling them apart leaks which emails are registered.
func (s *SigninService) Signin(email, password string) (*Session, error) {
	// Normalizing here keeps the lookup consistent with the email stored on
	// signup, which is lowercased and trimmed.
	email, err := ValidateEmail(email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	rec, err := s.users.GetUserByEmail(email)
	if err != nil {
		// A missing user is a credentials failure, not a not-found. Any other
		// database error propagates as-is; the handler maps it to a 500.
		if errors.Is(err, database.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := ComparePassword(password, rec.PasswordHash); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.startSession(rec.ID)
}

// startSession issues both halves together, so a client never ends up holding
// an access token it cannot renew.
func (s *SigninService) startSession(userID string) (*Session, error) {
	access, err := s.tokens.Issue(userID)
	if err != nil {
		return nil, err
	}

	refresh, refreshExpiresAt, err := s.refresh.Issue(userID)
	if err != nil {
		return nil, err
	}

	return &Session{
		AccessToken:      access,
		RefreshToken:     refresh,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}
