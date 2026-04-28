package http

import (
	"encoding/json"
	"net/http"

	"github.com/qoppa-tech/gitfed/internal/modules/session"
	"github.com/qoppa-tech/gitfed/pkg/logger"
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
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	result, err := p.totpSvc.Setup(r.Context(), userID.String(), userID.String())
	if err != nil {
		logger.FromContext(r.Context()).Error("totp setup failed", "step", "totp_setup", "user_id", userID.String(), "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, map[string]string{
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
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req totpVerifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil || req.Code == "" {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "code is required"})
		return
	}

	valid, err := p.totpSvc.Verify(r.Context(), userID.String(), req.Code)
	if err != nil {
		logger.FromContext(r.Context()).Error("totp verify failed", "step", "totp_verify", "user_id", userID.String(), "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if !valid {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "invalid totp code"})
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, map[string]string{"message": "totp verified"})
}
