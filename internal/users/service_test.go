package users

import (
	"link-shortner/internal/database"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

type fakeStore struct {
	record *database.UserRecord
}

func (s *fakeStore) CreateUser(rec *database.UserRecord) error {
	s.record = rec
	return nil
}

func (s *fakeStore) GetUserByEmail(string) (*database.UserRecord, error) { return nil, nil }

func (s *fakeStore) GetUserByID(string) (*database.UserRecord, error) { return nil, nil }

func TestCreateUserValidation(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{name: "", email: "ada@example.com", password: "password123", wantErr: ErrMissingName},
		{name: string(make([]byte, 101)), email: "ada@example.com", password: "password123", wantErr: ErrNameTooLong},
		{name: "Ada", email: "not-an-email", password: "password123", wantErr: ErrInvalidEmail},
		{name: "Ada", email: "ada@example.com", password: "short", wantErr: ErrPasswordTooShort},
	}

	for _, test := range tests {
		t.Run(test.name+test.email, func(t *testing.T) {
			service := NewService(&fakeStore{})
			if err := service.CreateUser(test.name, test.email, test.password); err != test.wantErr {
				t.Fatalf("CreateUser() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestCreateUserNormalizesInput(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	err := service.CreateUser("  Ada Lovelace  ", " ADA@EXAMPLE.COM ", "password123")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if store.record.Name != "Ada Lovelace" || store.record.Email != "ada@example.com" {
		t.Fatalf("record = %#v, want normalized name and email", store.record)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(store.record.PasswordHash), []byte("password123")); err != nil {
		t.Fatal("stored password is not a valid bcrypt hash for the submitted password")
	}
}
