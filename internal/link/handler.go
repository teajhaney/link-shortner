package link

import (
	"encoding/json"
	"errors"
	"link-shortner/internal/auth"
	"link-shortner/internal/database"
	"link-shortner/internal/response"
	"net/http"
)

type Handler struct {
	service *Service
	// protect wraps the routes that must only be reachable by an
	// authenticated caller. Injecting it keeps this package from depending on
	// how tokens happen to be verified, mirroring the users package.
	protect func(http.Handler) http.Handler
}

// NewHandler builds the link HTTP handler. protect is applied to the routes
// that create or expose a specific user's links: pass auth.Middleware in
// production, and an identity wrapper in tests that exercise the handlers
// themselves.
func NewHandler(linkService *Service, protect func(http.Handler) http.Handler) *Handler {
	return &Handler{service: linkService, protect: protect}
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	ShortURL string `json:"short_url"`
	Code     string `json:"code"`
	LongURL  string `json:"long_url"`
}

type linksResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    []LinkResult `json:"data"`
}

// HandleShorten creates a short link owned by the authenticated caller. The
// owner comes from the token in the context — never from the request body —
// so a client cannot create links on anybody else's behalf.
func (h *Handler) HandleShorten(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.service.Shorten(userID, req.URL)
	if err != nil {
		if errors.Is(err, ErrMissingURL) || errors.Is(err, ErrInvalidURL) || errors.Is(err, ErrInvalidOwner) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			response.WriteError(w, http.StatusInternalServerError, "Failed to save URL record")
		}
		return
	}

	response.WriteJSON(w, http.StatusCreated, shortenResponse{
		ShortURL: result.ShortURL,
		Code:     result.Code,
		LongURL:  result.LongURL,
	})
}

// HandleListLinks returns every link the authenticated caller has shortened.
// The owner filter happens in the store query, so the result is by
// construction only the caller's own links.
func (h *Handler) HandleListLinks(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	links, err := h.service.ListByUser(userID)
	if err != nil {
		if errors.Is(err, ErrInvalidOwner) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to retrieve links")
		return
	}

	response.WriteJSON(w, http.StatusOK, linksResponse{
		Code:    http.StatusOK,
		Message: "Links retrieved successfully",
		Data:    links,
	})
}

// HandleRedirect resolves a short code. It stays public and owner-blind on
// purpose: the whole point of a short link is that anybody who has the code
// can follow it, whether or not they have an account.
func (h *Handler) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	rec, err := h.service.Resolve(code)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "short link not found")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	http.Redirect(w, r, rec.LongURL, http.StatusFound)
}

// HandleStats returns a link's stats to its owner. A code that belongs to
// somebody else answers 404 — the same as a code that was never issued — so
// the caller learns nothing about codes they do not own.
func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	code := r.PathValue("code")

	result, err := h.service.StatsForUser(userID, code)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "short link not found")
			return
		}
		if errors.Is(err, ErrInvalidOwner) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	response.WriteJSON(w, http.StatusOK, result)
}
