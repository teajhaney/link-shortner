package users

import "net/http"

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/user/create", h.HandleSignup)
	mux.HandleFunc("GET /api/user", h.HandleGetUserByEmail)
	mux.HandleFunc("GET /api/user/{id}", h.HandleGetUserByID)

	// The routes below expose or change data about a user other than the
	// caller, so they require an authenticated request. Registering the
	// wrapped handler here keeps this package the single owner of its route
	// list, and avoids re-registering the same pattern in main, which the mux
	// rejects with a duplicate-pattern panic at startup.
	mux.Handle("PATCH /api/user/{id}", h.protect(http.HandlerFunc(h.HandleUpdateUser)))
	mux.Handle("DELETE /api/user/{id}", h.protect(http.HandlerFunc(h.HandleDeleteUser)))
	mux.Handle("GET /api/users", h.protect(http.HandlerFunc(h.HandleGetAllUsers)))
}
