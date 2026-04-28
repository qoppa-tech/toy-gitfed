package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/qoppa-tech/gitfed/internal/modules/session"
	"github.com/qoppa-tech/gitfed/internal/modules/user"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

const (
	accessCookieMaxAge  = 15 * 60          // 15 minutes in seconds
	refreshCookieMaxAge = 7 * 24 * 60 * 60 // 7 days in seconds
)

type SessionPresenter struct {
	sessionSvc *session.Service
	userSvc    *user.Service
	secure     bool
}

func NewSessionPresenter(sessionSvc *session.Service, userSvc *user.Service) *SessionPresenter {
	return &SessionPresenter{
		sessionSvc: sessionSvc,
		userSvc:    userSvc,
	}
}

func (p *SessionPresenter) SetSecure(secure bool) { p.secure = secure }

func (p *SessionPresenter) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", p.Login)
	mux.HandleFunc("POST /auth/logout", p.Logout)
	mux.HandleFunc("POST /auth/refresh", p.Refresh)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         loginUserDTO `json:"user"`
}

type loginUserDTO struct {
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type refreshResponse struct {
	AccessToken string `json:"access_token"`
}

func (p *SessionPresenter) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	if req.Email == "" || req.Password == "" {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}
	if len(req.Email) > maxEmailLen {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "email too long"})
		return
	}
	if len(req.Password) > maxPasswordLen {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "password too long"})
		return
	}

	log := logger.FromContext(r.Context()).With("email", req.Email)

	u, err := p.userSvc.GetByEmail(r.Context(), req.Email)
	if errors.Is(err, user.ErrNotFound) {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		log.Error("login user lookup failed", "step", "user_lookup", "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	log = log.With("user_id", u.ID.String())

	if err := p.userSvc.VerifyPassword(u.Password, req.Password); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		log.Error("login password verify failed", "step", "password_verify", "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	pair, err := p.sessionSvc.Create(r.Context(), u.ID)
	if err != nil {
		log.Error("login session create failed", "step", "session_create", "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	SetAccessCookie(w, pair.AccessToken, accessCookieMaxAge, p.secure)
	SetRefreshCookie(w, pair.RefreshToken, refreshCookieMaxAge, p.secure)

	writeJSON(r.Context(), w, http.StatusOK, loginResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User: loginUserDTO{
			UserID:   u.ID.String(),
			Name:     u.Name,
			Username: u.Username,
			Email:    u.Email,
		},
	})
}

func (p *SessionPresenter) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := extractRefreshToken(r)
	if refreshToken == "" {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "missing refresh token"})
		return
	}

	accessToken, err := p.sessionSvc.Refresh(r.Context(), refreshToken)
	if err != nil {
		if errors.Is(err, session.ErrInvalidRefreshToken) {
			writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired refresh token"})
			return
		}
		logger.FromContext(r.Context()).Error("session refresh failed", "step", "session_refresh", "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	SetAccessCookie(w, accessToken, accessCookieMaxAge, p.secure)

	writeJSON(r.Context(), w, http.StatusOK, refreshResponse{
		AccessToken: accessToken,
	})
}

func (p *SessionPresenter) Logout(w http.ResponseWriter, r *http.Request) {
	refreshToken := extractRefreshToken(r)
	if refreshToken == "" {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "missing refresh token"})
		return
	}

	if err := p.sessionSvc.Revoke(r.Context(), refreshToken); err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		logger.FromContext(r.Context()).Error("session revoke failed", "step", "session_revoke", "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	ClearAccessCookie(w, p.secure)
	ClearRefreshCookie(w, p.secure)

	writeJSON(r.Context(), w, http.StatusOK, map[string]string{"message": "logged out"})
}

func extractRefreshToken(r *http.Request) string {
	if c, err := r.Cookie(RefreshCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		return ""
	}
	return token
}
