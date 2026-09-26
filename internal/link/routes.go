package link

import "net/http"

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Creating a link and looking at your own links or stats are owner-only
	// operations, so they go through the injected auth middleware; the owner
	// is read from the token in the handlers, never from the request body.
	mux.Handle("POST /api/shorten", h.protect(http.HandlerFunc(h.HandleShorten)))
	mux.Handle("GET /api/links", h.protect(http.HandlerFunc(h.HandleListLinks)))
	mux.Handle("GET /api/stats/{code}", h.protect(http.HandlerFunc(h.HandleStats)))

	// The redirect stays public and owner-blind: the entire point of a short
	// link is that anybody holding the code can follow it without an account.
	mux.HandleFunc("GET /{code}", h.HandleRedirect)
}
