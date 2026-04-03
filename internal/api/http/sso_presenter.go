package http

import (
	"errors"
	"log"
	"net/http"

	"github.com/qoppa-tech/toy-gitfed/internal/modules/session"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/sso"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/user"
)

type SSOPresenter struct {
	ssoSvc     *sso.Service
	userSvc    *user.Service
	sessionSvc *session.Service
	secure     bool
}

func NewSSOPresenter(ssoSvc *sso.Service, userSvc *user.Service, sessionSvc *session.Service) *SSOPresenter {
	return &SSOPresenter{
		ssoSvc:     ssoSvc,
		userSvc:    userSvc,
		sessionSvc: sessionSvc,
	}
}

func (p *SSOPresenter) SetSecure(secure bool) { p.secure = secure }

func (p *SSOPresenter) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/google", p.GoogleRedirect)
	mux.HandleFunc("GET /auth/google/callback", p.GoogleCallback)
}

func (p *SSOPresenter) GoogleRedirect(w http.ResponseWriter, r *http.Request) {
	url, err := p.ssoSvc.GoogleAuthURL(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (p *SSOPresenter) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing state or code"})
		return
	}

	info, err := p.ssoSvc.GoogleCallback(r.Context(), state, code)
	if err != nil {
		if errors.Is(err, sso.ErrInvalidState) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid oauth state"})
			return
		}
		log.Printf("google callback error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "oauth failed"})
		return
	}

	// Find existing user by email or create one.
	u, err := p.userSvc.GetByEmail(r.Context(), info.Email)
	if errors.Is(err, user.ErrNotFound) {
		u, err = p.userSvc.Register(r.Context(), user.RegisterInput{
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
	if _, err := p.ssoSvc.FindOrCreateSSO(r.Context(), u.ID, sso.ProviderGoogle, info.Name, info.Email); err != nil {
		log.Printf("sso link error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Create session.
	pair, err := p.sessionSvc.Create(r.Context(), u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	SetAccessCookie(w, pair.AccessToken, accessCookieMaxAge, p.secure)
	SetRefreshCookie(w, pair.RefreshToken, refreshCookieMaxAge, p.secure)

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"user_id":       u.ID.String(),
		"email":         u.Email,
		"name":          u.Name,
	})
}
