package users

import "net/http"

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/user/create", h.HandleSignup)
	mux.HandleFunc("GET /api/user", h.HandleGetUserByEmail)
	mux.HandleFunc("GET /api/user/{id}", h.HandleGetUserByID)
	mux.HandleFunc("PATCH /api/user/{id}", h.HandleUpdateUser)
	mux.HandleFunc("DELETE /api/user/{id}", h.HandleDeleteUser)
	mux.HandleFunc("GET /api/users", h.HandleGetAllUsers)
}
