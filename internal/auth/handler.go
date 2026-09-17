package auth

import (
	"encoding/json"
	"errors"
	"io"
	"link-shortner/internal/response"
	"net/http"
)

type Handler struct {
	signin      *SigninService
	refresh     *RefreshService
	revocations *TokenValidator
}

func NewHandler(signin *SigninService, refresh *RefreshService, revocations *TokenValidator) *Handler {
	return &Handler{signin: signin, refresh: refresh, revocations: revocations}
}

type signinRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// refreshRequest is the body of refresh and, optionally, logout. The refresh
// token travels in the body rather than a header because it is a credential of
// its own, sent to a single endpoint, and never attached to ordinary requests.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// sessionResponse is what signin and refresh return. "token" keeps the name
// existing clients already read for the access token.
type sessionResponse struct {
	Code         int    `json:"code"`
	Message      string `json:"message"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	// ExpiresIn is the access token's remaining life in seconds, so a client
	// knows when to call refresh without decoding the token.
	ExpiresIn int `json:"expires_in"`
}

func writeSession(w http.ResponseWriter, status int, message string, session *Session) {
	response.WriteJSON(w, status, sessionResponse{
		Code:         status,
		Message:      message,
		Token:        session.AccessToken,
		RefreshToken: session.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(tokenLifetime.Seconds()),
	})
}

// HandleSignin authenticates a user and returns an access token together with
// the refresh token that renews it.
func (h *Handler) HandleSignin(w http.ResponseWriter, r *http.Request) {
	var req signinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	session, err := h.signin.Signin(req.Email, req.Password)
	if err != nil {
		// A credentials failure is a 401. Anything else from the store is a
		// 500, and the fallback message avoids leaking the internal error.
		if errors.Is(err, ErrInvalidCredentials) {
			response.WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to sign in")
		return
	}

	writeSession(w, http.StatusOK, "Signin successful", session)
}

// HandleVerify checks a token and reports who it belongs to. It is a debugging
// aid: it lets a client confirm a token is still valid and read its user ID
// without hitting a protected route. It goes through the revocation list too,
// so a logged-out token reports as invalid rather than looking healthy.
func (h *Handler) HandleVerify(w http.ResponseWriter, r *http.Request) {
	claims, err := h.revocations.Verify(extractToken(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"code":    http.StatusOK,
		"message": "Token is valid",
		"user_id": claims.UserID,
	})
}

// HandleRefresh exchanges a refresh token for a fresh pair of tokens. The
// presented refresh token is rotated, so a copy of it stops working the moment
// this returns.
func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	session, err := h.refresh.Refresh(req.RefreshToken)
	if err != nil {
		// Expired, revoked, replayed, and never-issued are one answer to the
		// client: sign in again.
		if errors.Is(err, ErrInvalidRefreshToken) {
			response.WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to refresh token")
		return
	}

	writeSession(w, http.StatusOK, "Token refreshed", session)
}

// HandleLogout ends a session. It revokes the access token if one is presented
// and the refresh token if one is supplied in the body, so a client that has
// already lost its access token can still kill the refresh token.
//
// It is deliberately forgiving: an unknown, already revoked, or expired
// credential is not an error, and signing out twice succeeds. The only failure
// is a request that presents nothing to revoke.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	// The body is optional, so an empty one is expected rather than an error.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	revoked := false

	if token := extractToken(r); token != "" {
		claims, err := h.revocations.Verify(token)
		switch {
		case err == nil:
			if err := h.revocations.Revoke(claims); err != nil {
				response.WriteError(w, http.StatusInternalServerError, "Failed to sign out")
				return
			}
			revoked = true
		case errors.Is(err, ErrExpiredToken):
			// An expired access token is already unusable, and refusing to
			// sign out because of it would strand the refresh token below.
		default:
			// A malformed or already revoked access token is likewise not a
			// reason to fail: the request still has the refresh token to
			// revoke, if the client sent one.
		}
	}

	if req.RefreshToken != "" {
		if err := h.refresh.Revoke(req.RefreshToken); err != nil {
			response.WriteError(w, http.StatusInternalServerError, "Failed to sign out")
			return
		}
		revoked = true
	}

	if !revoked {
		response.WriteError(w, http.StatusBadRequest, "provide a bearer token or a refresh token to revoke")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"code":    http.StatusOK,
		"message": "Signed out",
	})
}
