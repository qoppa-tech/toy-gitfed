package sso

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/qoppa-tech/toy-gitfed/internal/server/middleware"
	"github.com/qoppa-tech/toy-gitfed/internal/server/modules/session"
	"github.com/qoppa-tech/toy-gitfed/internal/server/modules/user"
)

const (
	accessCookieMaxAge  = 15 * 60          // 15 minutes
	refreshCookieMaxAge = 7 * 24 * 60 * 60 // 7 days
)

type Handler struct {
	ssoSvc     *Service
	userSvc    *user.Service
	sessionSvc *session.Service
	secure     bool
}

func NewHandler(ssoSvc *Service, userSvc *user.Service, sessionSvc *session.Service) *Handler {
	return &Handler{
		ssoSvc:     ssoSvc,
		userSvc:    userSvc,
		sessionSvc: sessionSvc,
	}
}

func (h *Handler) SetSecure(secure bool) { h.secure = secure }

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/google", h.GoogleRedirect)
	mux.HandleFunc("GET /auth/google/callback", h.GoogleCallback)
}

func (h *Handler) GoogleRedirect(w http.ResponseWriter, r *http.Request) {
	url, err := h.ssoSvc.GoogleAuthURL(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing state or code"})
		return
	}

	info, err := h.ssoSvc.GoogleCallback(r.Context(), state, code)
	if err != nil {
		if errors.Is(err, ErrInvalidState) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid oauth state"})
			return
		}
		log.Printf("google callback error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "oauth failed"})
		return
	}

	// Find existing user by email or create one.
	u, err := h.userSvc.GetByEmail(r.Context(), info.Email)
	if errors.Is(err, user.ErrNotFound) {
		u, err = h.userSvc.Register(r.Context(), user.RegisterInput{
			Name:     info.Name,
			Username: info.Email,
			Password: "",
			Email:    info.Email,
		})
	}
	if err != nil {
		log.Printf("user lookup/create error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Link SSO provider record.
	if _, err := h.ssoSvc.FindOrCreateSSO(r.Context(), u.UserID, "google", info.Name, info.Email); err != nil {
		log.Printf("sso link error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Create session with access + refresh tokens.
	pair, err := h.sessionSvc.Create(r.Context(), u.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Set HTTP-only cookies.
	middleware.SetAccessCookie(w, pair.AccessToken, accessCookieMaxAge, h.secure)
	middleware.SetRefreshCookie(w, pair.RefreshToken, refreshCookieMaxAge, h.secure)

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"user_id":       pgUUIDString(u.UserID),
		"email":         u.Email,
		"name":          u.Name,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func pgUUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	id, _ := uuid.FromBytes(u.Bytes[:])
	return id.String()
}
