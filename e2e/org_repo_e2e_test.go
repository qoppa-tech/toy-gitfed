package e2e

import (
	"io"
	"net/http"
	"strconv"
	"time"
)

type orgCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type orgResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type repoCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"is_private"`
	DefaultRef  string `json:"default_ref,omitempty"`
}

type repoResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsPrivate   bool   `json:"is_private"`
	OwnerID     string `json:"owner_id"`
	DefaultRef  string `json:"default_ref,omitempty"`
}

func (s *E2ESuite) TestOrg() {
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	_, loginResp := s.registerAndLogin("org", suffix)

	createReq := orgCreateRequest{
		Name:        "E2E Org " + suffix,
		Description: "org created by e2e",
	}

	createStatus, createBody := s.postJSON("/orgs", createReq, bearerHeaders(loginResp.AccessToken))
	s.Require().Equal(http.StatusCreated, createStatus)

	var created orgResponse
	s.requireJSON(createBody, &created)
	s.Require().NotEmpty(created.ID)
	s.Require().Equal(createReq.Name, created.Name)
	s.Require().Equal(createReq.Description, created.Description)

	listStatus, listBody := s.get("/orgs", bearerHeaders(loginResp.AccessToken), s.client)
	s.Require().Equal(http.StatusOK, listStatus)

	var listed []orgResponse
	s.requireJSON(listBody, &listed)
	s.Require().Len(listed, 1)
	s.Require().Equal(created.ID, listed[0].ID)
	s.Require().Equal(created.Name, listed[0].Name)
}

func (s *E2ESuite) TestRepo() {
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	registerReq, loginResp := s.registerAndLogin("repo", suffix)

	createReq := repoCreateRequest{
		Name:        "e2e-repo-" + suffix,
		Description: "repo created by e2e",
		IsPrivate:   true,
		DefaultRef:  "refs/heads/main",
	}

	createStatus, createBody := s.postJSON("/repos", createReq, bearerHeaders(loginResp.AccessToken))
	s.Require().Equal(http.StatusCreated, createStatus)

	var created repoResponse
	s.requireJSON(createBody, &created)
	s.Require().NotEmpty(created.ID)
	s.Require().Equal(createReq.Name, created.Name)
	s.Require().Equal(createReq.Description, created.Description)
	s.Require().Equal(createReq.IsPrivate, created.IsPrivate)
	s.Require().Equal(loginResp.User.UserID, created.OwnerID)
	s.Require().Equal(registerReq.Email, loginResp.User.Email)

	listStatus, listBody := s.get("/repos", bearerHeaders(loginResp.AccessToken), s.client)
	s.Require().Equal(http.StatusOK, listStatus)

	var listed []repoResponse
	s.requireJSON(listBody, &listed)
	s.Require().Len(listed, 1)
	s.Require().Equal(created.ID, listed[0].ID)

	deleteStatus, deleteBody := s.delete("/repos/"+created.ID, bearerHeaders(loginResp.AccessToken), s.client)
	s.Require().Equal(http.StatusNoContent, deleteStatus)
	s.Require().Len(deleteBody, 0)

	listAfterDeleteStatus, listAfterDeleteBody := s.get("/repos", bearerHeaders(loginResp.AccessToken), s.client)
	s.Require().Equal(http.StatusOK, listAfterDeleteStatus)

	var listedAfterDelete []repoResponse
	s.requireJSON(listAfterDeleteBody, &listedAfterDelete)
	s.Require().Len(listedAfterDelete, 0)
}

func (s *E2ESuite) registerAndLogin(prefix, suffix string) (authRegisterRequest, authLoginResponse) {
	password := "Passw0rd!"
	registerReq := authRegisterRequest{
		Name:     "E2E " + prefix + " " + suffix,
		Username: "e2e_" + prefix + "_" + suffix,
		Password: password,
		Email:    "e2e-" + prefix + "-" + suffix + "@example.com",
	}

	registerStatus, registerBody := s.postJSON("/auth/register", registerReq, nil)
	s.Require().Equal(http.StatusCreated, registerStatus)

	var registerResp authRegisterResponse
	s.requireJSON(registerBody, &registerResp)
	s.Require().NotEmpty(registerResp.UserID)

	loginReq := authLoginRequest{Email: registerReq.Email, Password: password}
	loginStatus, loginBody := s.postJSON("/auth/login", loginReq, nil)
	s.Require().Equal(http.StatusOK, loginStatus)

	var loginResp authLoginResponse
	s.requireJSON(loginBody, &loginResp)
	s.Require().NotEmpty(loginResp.AccessToken)
	s.Require().Equal(registerResp.UserID, loginResp.User.UserID)

	return registerReq, loginResp
}

func (s *E2ESuite) delete(path string, headers map[string]string, client *http.Client) (int, []byte) {
	req, err := http.NewRequest(http.MethodDelete, s.baseURL+path, nil)
	s.Require().NoError(err)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	return resp.StatusCode, body
}
