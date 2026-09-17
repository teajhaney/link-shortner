package auth

import (
	"errors"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrMissingName      = errors.New("name is required")
	ErrNameTooLong      = errors.New("name must be at most 100 characters")
	ErrInvalidEmail     = errors.New("email must be a valid email address")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must be at most 72 characters")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidID         = errors.New("invalid user ID")
)

const maxNameLength = 100

type SignupInput struct {
	Name     string
	Email    string
	Password string
}

// ValidateSignupInput validates and normalizes data submitted during signup.
func ValidateSignupInput(name, email, password string) (SignupInput, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))

	if name == "" {
		return SignupInput{}, ErrMissingName
	}
	if len(name) > maxNameLength {
		return SignupInput{}, ErrNameTooLong
	}
	if !IsValidEmail(email) {
		return SignupInput{}, ErrInvalidEmail
	}
	if len(password) < 8 {
		return SignupInput{}, ErrPasswordTooShort
	}
	if len(password) > 72 {
		return SignupInput{}, ErrPasswordTooLong
	}

	return SignupInput{Name: name, Email: email, Password: password}, nil
}

// HashPassword returns a bcrypt hash that is safe to store in the database.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ComparePassword reports whether password matches a stored bcrypt hash.
func ComparePassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func IsValidEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && strings.Contains(address.Address, "@")
}
