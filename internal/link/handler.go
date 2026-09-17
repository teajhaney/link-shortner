package link

import (
	"encoding/json"
	"errors"
	"link-shortner/internal/database"
	"link-shortner/internal/response"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(linkService *Service) *Handler {
	return &Handler{service: linkService}
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	ShortURL string `json:"short_url"`
	Code     string `json:"code"`
	LongURL  string `json:"long_url"`
}

func (h *Handler) HandleShorten(w http.ResponseWriter, r *http.Request) {
	var req shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.service.Shorten(req.URL)
	if err != nil {
		if errors.Is(err, ErrMissingURL) || errors.Is(err, ErrInvalidURL) {
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

func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	rec, err := h.service.Stats(code)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "short link not found")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	response.WriteJSON(w, http.StatusOK, rec)
}
