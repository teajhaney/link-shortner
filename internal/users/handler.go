package users

import (
	"encoding/json"
	"errors"
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

func (h *Handler) HandleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.service.CreateUser(req.Name, req.Email, req.Password)
	if err != nil {
		switch err {
		case ErrMissingName, ErrNameTooLong, ErrInvalidEmail, ErrPasswordTooShort, ErrPasswordTooLong:
			response.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			response.WriteError(w, http.StatusInternalServerError, "Failed to create user")
		}
		return
	}

	response.WriteJSON(w, http.StatusCreated, signupResponse{Code: http.StatusCreated, Message: "User created successfully"})
}

func (h *Handler) HandleGetUserByEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		response.WriteError(w, http.StatusBadRequest, "Missing email parameter")
		return
	}

	user, err := h.service.GetUserByEmail(email)
	if err != nil {
		switch err {
		case ErrInvalidEmail:
			response.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			response.WriteError(w, http.StatusInternalServerError, "Failed to retrieve user")
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, userResponse{
		Code:    http.StatusOK,
		Message: "User retrieved successfully",
		Data:    *user,
	})
}

func (h *Handler) HandleGetUserByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.WriteError(w, http.StatusBadRequest, "Missing user ID")
		return
	}

	user, err := h.service.GetUserByID(id)
	if err != nil {
		if errors.Is(err, database.ErrUserNotFound) {
			response.WriteError(w, http.StatusNotFound, "User not found")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to retrieve user")
		return
	}

	response.WriteJSON(w, http.StatusOK, userResponse{
		Code:    http.StatusOK,
		Message: "User retrieved successfully",
		Data:    *user,
	})
}

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
