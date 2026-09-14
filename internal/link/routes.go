package link

import "net/http"

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/shorten", h.HandleShorten)
	mux.HandleFunc("GET /api/stats/{code}", h.HandleStats)
	mux.HandleFunc("GET /{code}", h.HandleRedirect)
}
