package session

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/qoppa-tech/toy-gitfed/internal/server/middleware"
)

type TOTPHandler struct {
	totpSvc *TOTPService
}

func NewTOTPHandler(totpSvc *TOTPService) *TOTPHandler {
	return &TOTPHandler{totpSvc: totpSvc}
}

func (h *TOTPHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /auth/totp/setup", authMw(http.HandlerFunc(h.Setup)))
	mux.Handle("POST /auth/totp/verify", authMw(http.HandlerFunc(h.Verify)))
}

func (h *TOTPHandler) Setup(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	uid, _ := uuid.FromBytes(userID.Bytes[:])
	result, err := h.totpSvc.Setup(r.Context(), uid.String(), uid.String())
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

func (h *TOTPHandler) Verify(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req totpVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code is required"})
		return
	}

	uid, _ := uuid.FromBytes(userID.Bytes[:])
	valid, err := h.totpSvc.Verify(r.Context(), uid.String(), req.Code)
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
