package users

import (
	"encoding/json"
	"errors"
	"link-shortner/internal/auth"
	"link-shortner/internal/database"
	"link-shortner/internal/response"
	"net/http"
)

type Handler struct {
	service *service
}

func NewHandler(userService *service) *Handler {
	return &Handler{service: userService}
}

type signupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type signupResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type updateUserRequest struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Password *string `json:"password"`
}

type userResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    UserResult `json:"data"`
}

type usersResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    []*UserResult `json:"data"`
}

// Create user
func (h *Handler) HandleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.service.CreateUser(req.Name, req.Email, req.Password)
	if err != nil {
		handleUserError(w, err, "Failed to create user")
		return
	}

	response.WriteJSON(w, http.StatusCreated, signupResponse{Code: http.StatusCreated, Message: "User created successfully"})
}

// get user by email
func (h *Handler) HandleGetUserByEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")

	if email == "" {
		response.WriteError(w, http.StatusBadRequest, "Missing email parameter")
		return
	}
	if !auth.IsValidEmail(email) {
		response.WriteError(w, http.StatusBadRequest, "Invalid email parameter")
		return
	}
	user, err := h.service.GetUserByEmail(email)
	if err != nil {
		handleUserError(w, err, "Failed to retrieve user")
		return
	}

	response.WriteJSON(w, http.StatusOK, userResponse{
		Code:    http.StatusOK,
		Message: "User retrieved successfully",
		Data:    *user,
	})
}

// Get by user ID
func (h *Handler) HandleGetUserByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if id == "" {
		response.WriteError(w, http.StatusBadRequest, "Missing id parameter")
		return
	}

	user, err := h.service.GetUserByID(id)
	if err != nil {
		handleUserError(w, err, "Failed to retrieve user")
		return
	}

	response.WriteJSON(w, http.StatusOK, userResponse{
		Code:    http.StatusOK,
		Message: "User retrieved successfully",
		Data:    *user,
	})
}

// Get all users
func (h *Handler) HandleGetAllUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.GetAllUsers()
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to retrieve users")
		return
	}

	response.WriteJSON(w, http.StatusOK, usersResponse{
		Code:    http.StatusOK,
		Message: "Users retrieved successfully",
		Data:    users,
	})
}

func (h *Handler) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.service.UpdateUser(r.PathValue("id"), req.Name, req.Email, req.Password)
	if err != nil {
		handleUserError(w, err, "Failed to update user")
		return
	}

	response.WriteJSON(w, http.StatusOK, userResponse{
		Code:    http.StatusOK,
		Message: "User updated successfully",
		Data:    *user,
	})
}

func (h *Handler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteUser(r.PathValue("id")); err != nil {
		handleUserError(w, err, "Failed to delete user")
		return
	}

	response.WriteJSON(w, http.StatusOK, signupResponse{
		Code:    http.StatusOK,
		Message: "User deleted successfully",
	})
}

// handleUserError maps the service's sentinel errors onto HTTP status codes.
// Validation failures are client errors (400), duplicate emails are conflicts
// (409), and a missing user is a not found (404); anything else is an internal
// error and the caller supplies a non-leaking fallback message.
func handleUserError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrMissingName),
		errors.Is(err, ErrNameTooLong),
		errors.Is(err, ErrInvalidEmail),
		errors.Is(err, ErrPasswordTooShort),
		errors.Is(err, ErrPasswordTooLong),
		errors.Is(err, ErrInvalidID),
		errors.Is(err, ErrNoUpdateFields):
		response.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, database.ErrEmailConflict):
		response.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, database.ErrUserNotFound):
		response.WriteError(w, http.StatusNotFound, "User not found")
	default:
		response.WriteError(w, http.StatusInternalServerError, fallback)
	}
}
