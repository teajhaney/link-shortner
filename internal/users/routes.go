package users

import "net/http"

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/user/create", h.HandleSignup)

	// Safe to protect: sign-in never goes through this HTTP route —
	// auth.SigninService looks the email up in the database store directly,
	// so locking these endpoints down cannot break authentication. Every
	// route below is self-only: an authenticated caller may only read,
	// update, or delete their own record (checked in the handlers).
	mux.Handle("GET /api/user", h.protect(http.HandlerFunc(h.HandleGetUserByEmail)))
	mux.Handle("GET /api/user/{id}", h.protect(http.HandlerFunc(h.HandleGetUserByID)))

	// The update and delete routes are wrapped by the auth middleware, which
	// puts the requester's ID in context; the handlers then reject anyone
	// whose ID does not match the record being changed. Registering the
	// wrapped handlers here keeps this package the single owner of its route
	// list, and avoids re-registering the same patterns in main, which the
	// mux rejects with a duplicate-pattern panic at startup.
	mux.Handle("PATCH /api/user/{id}", h.protect(http.HandlerFunc(h.HandleUpdateUser)))
	mux.Handle("DELETE /api/user/{id}", h.protect(http.HandlerFunc(h.HandleDeleteUser)))

	// Disabled on purpose: without an admin role, an authenticated user
	// listing every account (name and email) is an information leak — and it
	// would hand out the IDs that the self-only checks above rely on. Re-add
	// behind a role check when admin support exists.
	// mux.Handle("GET /api/users", h.protect(http.HandlerFunc(h.HandleGetAllUsers)))
}
