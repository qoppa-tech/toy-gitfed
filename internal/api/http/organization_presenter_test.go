package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/qoppa-tech/gitfed/internal/modules/organization"
)

type orgTokenValidator struct {
	userID uuid.UUID
	err    error
}

func (v *orgTokenValidator) Validate(_ context.Context, _ string) (uuid.UUID, error) {
	if v.err != nil {
		return uuid.Nil, v.err
	}
	return v.userID, nil
}

type fakeOrganizationRepository struct {
	orgs        map[uuid.UUID]organization.Organization
	memberships map[uuid.UUID]map[uuid.UUID]struct{}
	failAddUser error
}

func newFakeOrganizationRepository() *fakeOrganizationRepository {
	return &fakeOrganizationRepository{
		orgs:        make(map[uuid.UUID]organization.Organization),
		memberships: make(map[uuid.UUID]map[uuid.UUID]struct{}),
	}
}

func (r *fakeOrganizationRepository) Create(_ context.Context, org organization.Organization) (organization.Organization, error) {
	r.orgs[org.ID] = org
	return org, nil
}

func (r *fakeOrganizationRepository) GetByID(_ context.Context, id uuid.UUID) (organization.Organization, error) {
	org, ok := r.orgs[id]
	if !ok {
		return organization.Organization{}, organization.ErrNotFound
	}
	return org, nil
}

func (r *fakeOrganizationRepository) DeleteByID(_ context.Context, id uuid.UUID) error {
	delete(r.orgs, id)
	for userID := range r.memberships {
		delete(r.memberships[userID], id)
	}
	return nil
}

func (r *fakeOrganizationRepository) AddUser(_ context.Context, orgID, userID uuid.UUID) error {
	if r.failAddUser != nil {
		err := r.failAddUser
		r.failAddUser = nil
		return err
	}
	if _, ok := r.orgs[orgID]; !ok {
		return organization.ErrNotFound
	}
	if _, ok := r.memberships[userID]; !ok {
		r.memberships[userID] = make(map[uuid.UUID]struct{})
	}
	r.memberships[userID][orgID] = struct{}{}
	return nil
}

func (r *fakeOrganizationRepository) RemoveUser(_ context.Context, orgID, userID uuid.UUID) error {
	if _, ok := r.memberships[userID]; !ok {
		return nil
	}
	delete(r.memberships[userID], orgID)
	return nil
}

func (r *fakeOrganizationRepository) GetByUserID(_ context.Context, userID uuid.UUID) ([]organization.Organization, error) {
	orgIDs := r.memberships[userID]
	orgs := make([]organization.Organization, 0, len(orgIDs))
	for orgID := range orgIDs {
		orgs = append(orgs, r.orgs[orgID])
	}
	return orgs, nil
}

func newOrganizationTestMux(repo organization.Repository, authUserID uuid.UUID) *http.ServeMux {
	svc := organization.NewService(repo)
	presenter := NewOrganizationPresenter(svc)
	mux := http.NewServeMux()
	presenter.RegisterRoutes(mux, Auth(&orgTokenValidator{userID: authUserID}))
	return mux
}

func TestOrganizationPresenter_UnauthorizedRoutes(t *testing.T) {
	repo := newFakeOrganizationRepository()
	mux := newOrganizationTestMux(repo, uuid.New())

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create organization", method: http.MethodPost, path: "/orgs", body: `{"name":"Acme"}`},
		{name: "list organizations", method: http.MethodGet, path: "/orgs"},
		{name: "add user to organization", method: http.MethodPost, path: "/orgs/11111111-1111-1111-1111-111111111111/users", body: `{"user_id":"22222222-2222-2222-2222-222222222222"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestOrganizationPresenter_CreateAndListFlow(t *testing.T) {
	authUserID := uuid.New()
	repo := newFakeOrganizationRepository()
	mux := newOrganizationTestMux(repo, authUserID)

	createReq := httptest.NewRequest(http.MethodPost, "/orgs", bytes.NewBufferString(`{"name":"Acme","description":"Example org"}`))
	createReq.Header.Set("Authorization", "Bearer test-token")
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()

	mux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}

	var created map[string]string
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created["name"] != "Acme" {
		t.Fatalf("created name = %q, want %q", created["name"], "Acme")
	}
	if created["id"] == "" {
		t.Fatal("created id is empty")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/orgs", nil)
	listReq.Header.Set("Authorization", "Bearer test-token")
	listRec := httptest.NewRecorder()

	mux.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	var listed []map[string]string
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed length = %d, want %d", len(listed), 1)
	}
	if listed[0]["id"] != created["id"] {
		t.Fatalf("listed id = %q, want %q", listed[0]["id"], created["id"])
	}
}

func TestOrganizationPresenter_Create_CompensatesWhenCreatorMembershipFails(t *testing.T) {
	authUserID := uuid.New()
	repo := newFakeOrganizationRepository()
	repo.failAddUser = errors.New("add user failed")
	mux := newOrganizationTestMux(repo, authUserID)

	req := httptest.NewRequest(http.MethodPost, "/orgs", bytes.NewBufferString(`{"name":"Acme"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if len(repo.orgs) != 0 {
		t.Fatalf("org count = %d, want %d", len(repo.orgs), 0)
	}
}

func TestOrganizationPresenter_AddUser_InvalidIDs(t *testing.T) {
	authUserID := uuid.New()
	repo := newFakeOrganizationRepository()
	mux := newOrganizationTestMux(repo, authUserID)

	orgID := uuid.New()
	_, err := repo.Create(context.Background(), organization.Organization{ID: orgID, Name: "Acme"})
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}

	t.Run("invalid org id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgs/not-a-uuid/users", bytes.NewBufferString(`{"user_id":"11111111-1111-1111-1111-111111111111"}`))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid user id", func(t *testing.T) {
		path := "/orgs/" + orgID.String() + "/users"
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"user_id":"not-a-uuid"}`))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestOrganizationPresenter_AddUser_Success(t *testing.T) {
	authUserID := uuid.New()
	repo := newFakeOrganizationRepository()
	mux := newOrganizationTestMux(repo, authUserID)

	orgID := uuid.New()
	_, err := repo.Create(context.Background(), organization.Organization{ID: orgID, Name: "Acme"})
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	if err := repo.AddUser(context.Background(), orgID, authUserID); err != nil {
		t.Fatalf("seed auth user membership: %v", err)
	}

	userID := uuid.New()
	path := "/orgs/" + orgID.String() + "/users"
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"user_id":"`+userID.String()+`"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	orgs, err := repo.GetByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("read memberships: %v", err)
	}
	if len(orgs) != 1 || orgs[0].ID != orgID {
		t.Fatalf("memberships = %+v, want one org %s", orgs, orgID)
	}
}

func TestOrganizationPresenter_AddUser_ForbiddenWhenAuthUserNotInOrg(t *testing.T) {
	authUserID := uuid.New()
	repo := newFakeOrganizationRepository()
	mux := newOrganizationTestMux(repo, authUserID)

	orgID := uuid.New()
	_, err := repo.Create(context.Background(), organization.Organization{ID: orgID, Name: "Acme"})
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}

	userID := uuid.New()
	path := "/orgs/" + orgID.String() + "/users"
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"user_id":"`+userID.String()+`"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestOrganizationPresenter_AddUser_MissingOrgReturnsForbidden(t *testing.T) {
	authUserID := uuid.New()
	repo := newFakeOrganizationRepository()
	mux := newOrganizationTestMux(repo, authUserID)

	memberOrgID := uuid.New()
	_, err := repo.Create(context.Background(), organization.Organization{ID: memberOrgID, Name: "Member Org"})
	if err != nil {
		t.Fatalf("seed member organization: %v", err)
	}
	if err := repo.AddUser(context.Background(), memberOrgID, authUserID); err != nil {
		t.Fatalf("seed auth user membership: %v", err)
	}

	missingOrgID := uuid.New()
	userID := uuid.New()
	path := "/orgs/" + missingOrgID.String() + "/users"
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"user_id":"`+userID.String()+`"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestOrganizationTokenValidator_Error(t *testing.T) {
	v := &orgTokenValidator{err: errors.New("boom")}
	_, err := v.Validate(context.Background(), "token")
	if err == nil {
		t.Fatal("expected error")
	}
}
