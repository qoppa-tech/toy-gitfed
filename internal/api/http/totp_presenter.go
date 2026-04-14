package http

import (
	"encoding/json"
	"net/http"

	"github.com/qoppa-tech/toy-gitfed/internal/modules/session"
)

type TOTPPresenter struct {
	totpSvc *session.TOTPService
}

func NewTOTPPresenter(totpSvc *session.TOTPService) *TOTPPresenter {
	return &TOTPPresenter{totpSvc: totpSvc}
}

func (p *TOTPPresenter) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /auth/totp/setup", authMw(http.HandlerFunc(p.Setup)))
	mux.Handle("POST /auth/totp/verify", authMw(http.HandlerFunc(p.Verify)))
}

func (p *TOTPPresenter) Setup(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	result, err := p.totpSvc.Setup(r.Context(), userID.String(), userID.String())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"secret": result.Secret,
		"url":    result.URL,
	})
}

type totpVerifyRequest struct {
	Code string `json:"code"`
}

func (p *TOTPPresenter) Verify(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req totpVerifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil || req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code is required"})
		return
	}

	valid, err := p.totpSvc.Verify(r.Context(), userID.String(), req.Code)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid totp code"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "totp verified"})
}
