package users

import (
	"link-shortner/internal/auth"
	"link-shortner/internal/database"
	"time"
)

var (
	ErrMissingName      = auth.ErrMissingName
	ErrNameTooLong      = auth.ErrNameTooLong
	ErrInvalidEmail     = auth.ErrInvalidEmail
	ErrPasswordTooShort = auth.ErrPasswordTooShort
	ErrPasswordTooLong  = auth.ErrPasswordTooLong
	ErrInvalidID        = auth.ErrInvalidID
	ErrUserNotFound     = auth.ErrUserNotFound
)

type service struct {
	database database.Users
}

// UserResult is the safe user data returned to handlers and clients.
// It intentionally excludes the password and password hash.
type UserResult struct {
	ID        string
	Name      string
	Email     string
	CreatedAt string
	UpdatedAt string
}

func NewService(storage database.Users) *service {
	return &service{database: storage}
}

func (s *service) CreateUser(name, email, password string) error {
	input, err := auth.ValidateSignupInput(name, email, password)
	if err != nil {
		return err
	}

	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return err
	}

	now := time.Now()
	rec := &database.UserRecord{
		Name:         input.Name,
		Email:        input.Email,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return s.database.CreateUser(rec)
}

func (s *service) GetUserByEmail(email string) (*UserResult, error) {
	rec, err := s.database.GetUserByEmail(email)
	if err != nil {
		return nil, err
	}

	return &UserResult{
		ID:        rec.ID,
		Name:      rec.Name,
		Email:     rec.Email,
		CreatedAt: rec.CreatedAt.Format(time.RFC3339),
		UpdatedAt: rec.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *service) GetUserByID(id string) (*UserResult, error) {
	rec, err := s.database.GetUserByID(id)
	if err != nil {
		return nil, err
	}

	return &UserResult{
		ID:        rec.ID,
		Name:      rec.Name,
		Email:     rec.Email,
		CreatedAt: rec.CreatedAt.Format(time.RFC3339),
		UpdatedAt: rec.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *service) GetAllUsers() ([]*UserResult, error) {
	recs, err := s.database.GetAllUsers()
	if err != nil {
		return nil, err
	}

	var results []*UserResult
	for _, rec := range recs {
		results = append(results, &UserResult{
			ID:        rec.ID,
			Name:      rec.Name,
			Email:     rec.Email,
			CreatedAt: rec.CreatedAt.Format(time.RFC3339),
			UpdatedAt: rec.UpdatedAt.Format(time.RFC3339),
		})
	}

	return results, nil
}
