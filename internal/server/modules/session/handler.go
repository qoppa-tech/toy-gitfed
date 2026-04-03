package session

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/qoppa-tech/toy-gitfed/internal/server/middleware"
	"github.com/qoppa-tech/toy-gitfed/internal/server/modules/user"
)

const (
	maxEmailLen       = 255
	maxPasswordLen    = 72 // bcrypt limit
	accessCookieMaxAge  = 15 * 60     // 15 minutes in seconds
	refreshCookieMaxAge = 7 * 24 * 60 * 60 // 7 days in seconds
)

type Handler struct {
	sessionSvc *Service
	userSvc    *user.Service
	secure     bool // true in production (HTTPS), false for local dev
}

func NewHandler(sessionSvc *Service, userSvc *user.Service) *Handler {
	return &Handler{
		sessionSvc: sessionSvc,
		userSvc:    userSvc,
	}
}

func (h *Handler) SetSecure(secure bool) { h.secure = secure }

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/logout", h.Logout)
	mux.HandleFunc("POST /auth/refresh", h.Refresh)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         userResponse `json:"user"`
}

type refreshResponse struct {
	AccessToken string `json:"access_token"`
}

type userResponse struct {
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}
	if len(req.Email) > maxEmailLen {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email too long"})
		return
	}
	if len(req.Password) > maxPasswordLen {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password too long"})
		return
	}

	u, err := h.userSvc.GetByEmail(r.Context(), req.Email)
	if errors.Is(err, user.ErrNotFound) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := h.userSvc.VerifyPassword(u.Password, req.Password); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	pair, err := h.sessionSvc.Create(r.Context(), u.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Set HTTP-only cookies.
	middleware.SetAccessCookie(w, pair.AccessToken, accessCookieMaxAge, h.secure)
	middleware.SetRefreshCookie(w, pair.RefreshToken, refreshCookieMaxAge, h.secure)

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User: userResponse{
			UserID:   pgUUIDString(u.UserID),
			Name:     u.Name,
			Username: u.Username,
			Email:    u.Email,
		},
	})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := extractRefreshToken(r)
	if refreshToken == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing refresh token"})
		return
	}

	accessToken, err := h.sessionSvc.Refresh(r.Context(), refreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired refresh token"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	middleware.SetAccessCookie(w, accessToken, accessCookieMaxAge, h.secure)

	writeJSON(w, http.StatusOK, refreshResponse{
		AccessToken: accessToken,
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	refreshToken := extractRefreshToken(r)
	if refreshToken == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing refresh token"})
		return
	}

	if err := h.sessionSvc.Revoke(r.Context(), refreshToken); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Clear both cookies.
	middleware.ClearAccessCookie(w, h.secure)
	middleware.ClearRefreshCookie(w, h.secure)

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// extractRefreshToken checks the refresh_token cookie first, then request body/header.
func extractRefreshToken(r *http.Request) string {
	if c, err := r.Cookie(middleware.RefreshCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	return extractBearerToken(r)
}

func extractBearerToken(r *http.Request) string {
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		return ""
	}
	return token
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
