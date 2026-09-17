package auth

import (
	"errors"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrMissingName        = errors.New("name is required")
	ErrNameTooLong        = errors.New("name must be at most 100 characters")
	ErrInvalidEmail       = errors.New("email must be a valid email address")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong    = errors.New("password must be at most 72 characters")
	ErrNoUpdateFields     = errors.New("provide at least one field to update")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidID          = errors.New("invalid user ID")
)

const maxNameLength = 100

type SignupInput struct {
	Name     string
	Email    string
	Password string
}

// ValidateSignupInput validates and normalizes data submitted during signup.
func ValidateSignupInput(name, email, password string) (SignupInput, error) {
	var err error
	if name, err = ValidateName(name); err != nil {
		return SignupInput{}, err
	}
	if email, err = ValidateEmail(email); err != nil {
		return SignupInput{}, err
	}
	if err = ValidatePassword(password); err != nil {
		return SignupInput{}, err
	}

	return SignupInput{Name: name, Email: email, Password: password}, nil
}

func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrMissingName
	}
	if len(name) > maxNameLength {
		return "", ErrNameTooLong
	}
	return name, nil
}

func ValidateEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !IsValidEmail(email) {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if len(password) > 72 {
		return ErrPasswordTooLong
	}
	return nil
}

func ValidateUserID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if len(id) != 36 {
		return "", ErrInvalidID
	}

	for index, char := range id {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return "", ErrInvalidID
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return "", ErrInvalidID
		}
	}

	return id, nil
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
