package users

import "net/http"

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/users/create", h.HandleSignup)
	mux.HandleFunc("GET /api/users", h.HandleGetUserByEmail)
	mux.HandleFunc("GET /api/users/{id}", h.HandleGetUserByID)
}
