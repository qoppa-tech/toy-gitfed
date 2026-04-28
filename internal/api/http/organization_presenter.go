package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/qoppa-tech/gitfed/internal/modules/organization"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

type OrganizationPresenter struct {
	service *organization.Service
}

func NewOrganizationPresenter(service *organization.Service) *OrganizationPresenter {
	return &OrganizationPresenter{service: service}
}

func (p *OrganizationPresenter) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /orgs", authMw(http.HandlerFunc(p.Create)))
	mux.Handle("GET /orgs", authMw(http.HandlerFunc(p.List)))
	mux.Handle("POST /orgs/{orgID}/users", authMw(http.HandlerFunc(p.AddUser)))
}

type createOrganizationRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type addOrganizationUserRequest struct {
	UserID string `json:"user_id"`
}

type organizationResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (p *OrganizationPresenter) Create(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createOrganizationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	log := logger.FromContext(r.Context()).With("auth_user_id", authUserID.String())

	org, err := p.service.Create(r.Context(), organization.CreateInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		log.Error("organization create failed", "step", "org_create", "org_name", req.Name, "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := p.ensureCreatorMembership(r.Context(), org.ID, authUserID); err != nil {
		log.Error("organization creator membership failed", "step", "org_creator_membership", "org_id", org.ID.String(), "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, toOrganizationResponse(org))
}

func (p *OrganizationPresenter) List(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgs, err := p.service.GetByUserID(r.Context(), authUserID)
	if err != nil {
		logger.FromContext(r.Context()).Error("organization list failed", "step", "org_list", "auth_user_id", authUserID.String(), "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]organizationResponse, len(orgs))
	for i := range orgs {
		resp[i] = toOrganizationResponse(orgs[i])
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (p *OrganizationPresenter) AddUser(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := uuid.Parse(r.PathValue("orgID"))
	if err != nil {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	var req addOrganizationUserRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	userID, err := uuid.Parse(strings.TrimSpace(req.UserID))
	if err != nil {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	log := logger.FromContext(r.Context()).
		With("auth_user_id", authUserID.String()).
		With("org_id", orgID.String()).
		With("target_user_id", userID.String())

	authUserOrgs, err := p.service.GetByUserID(r.Context(), authUserID)
	if err != nil {
		log.Error("organization membership lookup failed", "step", "org_auth_lookup", "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	// Authorize before disclosing existence details to avoid org enumeration.
	if !userBelongsToOrg(authUserOrgs, orgID) {
		writeJSON(r.Context(), w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	if err := p.service.AddUser(r.Context(), orgID, userID); err != nil {
		if errors.Is(err, organization.ErrNotFound) {
			writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "organization not found"})
			return
		}
		log.Error("organization add user failed", "step", "org_add_user", "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, map[string]string{"message": "user added"})
}

func (p *OrganizationPresenter) ensureCreatorMembership(ctx context.Context, orgID, userID uuid.UUID) error {
	if err := p.service.AddUser(ctx, orgID, userID); err != nil {
		if rollbackErr := p.service.DeleteByID(ctx, orgID); rollbackErr != nil {
			logger.FromContext(ctx).Error("organization rollback failed after membership error", "step", "org_rollback", "org_id", orgID.String(), "user_id", userID.String(), "add_user_error", err, "delete_error", rollbackErr)
		}
		return err
	}
	return nil
}

func userBelongsToOrg(orgs []organization.Organization, orgID uuid.UUID) bool {
	for _, org := range orgs {
		if org.ID == orgID {
			return true
		}
	}
	return false
}

func toOrganizationResponse(org organization.Organization) organizationResponse {
	return organizationResponse{
		ID:          org.ID.String(),
		Name:        org.Name,
		Description: org.Description,
	}
}
