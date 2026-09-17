package auth

import "net/http"

// RegisterRoutes mounts the auth endpoints on the mux.
//
// Signin is public: it is how a client obtains a token in the first place.
// Refresh is public too, because the whole point of a refresh token is that it
// works after the access token has expired. Verify is public so a client can
// check a token's validity and read its user ID without hitting a protected
// route. Logout is public as well: it revokes whatever credential it is given,
// including when the access token is already expired.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/signin", h.HandleSignin)
	mux.HandleFunc("POST /api/auth/refresh", h.HandleRefresh)
	mux.HandleFunc("POST /api/auth/logout", h.HandleLogout)
	mux.HandleFunc("GET /api/auth/verify", h.HandleVerify)
}
