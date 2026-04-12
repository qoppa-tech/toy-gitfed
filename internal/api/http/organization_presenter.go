package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/organization"
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
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createOrganizationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	org, err := p.service.Create(r.Context(), organization.CreateInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := p.ensureCreatorMembership(r.Context(), org.ID, authUserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toOrganizationResponse(org))
}

func (p *OrganizationPresenter) List(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgs, err := p.service.GetByUserID(r.Context(), authUserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]organizationResponse, len(orgs))
	for i := range orgs {
		resp[i] = toOrganizationResponse(orgs[i])
	}

	writeJSON(w, http.StatusOK, resp)
}

func (p *OrganizationPresenter) AddUser(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID, err := uuid.Parse(r.PathValue("orgID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	var req addOrganizationUserRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	userID, err := uuid.Parse(strings.TrimSpace(req.UserID))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	authUserOrgs, err := p.service.GetByUserID(r.Context(), authUserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	// Authorize before disclosing existence details to avoid org enumeration.
	if !userBelongsToOrg(authUserOrgs, orgID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	if err := p.service.AddUser(r.Context(), orgID, userID); err != nil {
		if errors.Is(err, organization.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "user added"})
}

func (p *OrganizationPresenter) ensureCreatorMembership(ctx context.Context, orgID, userID uuid.UUID) error {
	if err := p.service.AddUser(ctx, orgID, userID); err != nil {
		_ = p.service.DeleteByID(ctx, orgID)
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
