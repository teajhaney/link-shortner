package users

import (
	"link-shortner/internal/database"
	"testing"
	"time"

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

func (s *fakeStore) GetAllUsers() ([]database.UserRecord, error) { return nil, nil }

func (s *fakeStore) UpdateUser(_ string, update database.UserUpdate) (*database.UserRecord, error) {
	if s.record == nil {
		s.record = &database.UserRecord{ID: "550e8400-e29b-41d4-a716-446655440000"}
	}
	if update.Name != nil {
		s.record.Name = *update.Name
	}
	if update.Email != nil {
		s.record.Email = *update.Email
	}
	if update.PasswordHash != nil {
		s.record.PasswordHash = *update.PasswordHash
	}
	s.record.UpdatedAt = update.UpdatedAt
	return s.record, nil
}

func (s *fakeStore) DeleteUser(string) error { return nil }

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

func TestUpdateUserHashesNewPassword(t *testing.T) {
	store := &fakeStore{record: &database.UserRecord{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Name:      "Ada",
		Email:     "ada@example.com",
		CreatedAt: time.Now(),
	}}
	service := NewService(store)
	password := "new-password123"

	result, err := service.UpdateUser(store.record.ID, nil, nil, &password)
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if result.ID != store.record.ID {
		t.Fatalf("UpdateUser() ID = %q, want %q", result.ID, store.record.ID)
	}
	if store.record.PasswordHash == password {
		t.Fatal("UpdateUser() stored the raw password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(store.record.PasswordHash), []byte(password)); err != nil {
		t.Fatal("UpdateUser() did not store a valid bcrypt hash")
	}
}

func TestUpdateUserRequiresAField(t *testing.T) {
	service := NewService(&fakeStore{})
	_, err := service.UpdateUser("550e8400-e29b-41d4-a716-446655440000", nil, nil, nil)
	if err != ErrNoUpdateFields {
		t.Fatalf("UpdateUser() error = %v, want %v", err, ErrNoUpdateFields)
	}
}
