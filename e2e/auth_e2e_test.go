package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type authRegisterRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type authLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authRegisterResponse struct {
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type authLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         struct {
		UserID   string `json:"user_id"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user"`
}

type authRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type authErrorResponse struct {
	Error string `json:"error"`
}

type authLogoutResponse struct {
	Message string `json:"message"`
}

func (s *E2ESuite) TestAuthSessionLifecycle() {
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	password := "Passw0rd!"

	registerReq := authRegisterRequest{
		Name:     "E2E Auth " + suffix,
		Username: "e2e_auth_" + suffix,
		Password: password,
		Email:    "e2e-auth-" + suffix + "@example.com",
	}

	registerStatus, registerBody := s.postJSON("/auth/register", registerReq, nil)
	s.Require().Equal(http.StatusCreated, registerStatus)

	var registerResp authRegisterResponse
	s.requireJSON(registerBody, &registerResp)
	s.Require().NotEmpty(registerResp.UserID)
	s.Require().Equal(registerReq.Name, registerResp.Name)
	s.Require().Equal(registerReq.Username, registerResp.Username)
	s.Require().Equal(registerReq.Email, registerResp.Email)

	loginReq := authLoginRequest{Email: registerReq.Email, Password: password}
	loginStatus, loginBody := s.postJSON("/auth/login", loginReq, nil)
	s.Require().Equal(http.StatusOK, loginStatus)

	var loginResp authLoginResponse
	s.requireJSON(loginBody, &loginResp)
	s.Require().NotEmpty(loginResp.AccessToken)
	s.Require().NotEmpty(loginResp.RefreshToken)
	s.Require().Equal(registerResp.UserID, loginResp.User.UserID)
	s.Require().Equal(registerReq.Email, loginResp.User.Email)
	activeRefreshToken := loginResp.RefreshToken

	headerClient := &http.Client{Timeout: 10 * time.Second}

	refreshStatus, refreshBody, refreshCookies := s.postJSONWithClient(headerClient, "/auth/refresh", nil, bearerHeaders(activeRefreshToken))
	s.Require().Equal(http.StatusOK, refreshStatus)

	var refreshResp authRefreshResponse
	s.requireJSON(refreshBody, &refreshResp)
	s.Require().NotEmpty(refreshResp.AccessToken)
	s.Require().NotEqual(loginResp.AccessToken, refreshResp.AccessToken)
	activeRefreshToken = nextRefreshToken(activeRefreshToken, refreshResp, refreshCookies)

	logoutStatus, logoutBody, _ := s.postJSONWithClient(headerClient, "/auth/logout", nil, bearerHeaders(activeRefreshToken))
	s.Require().Equal(http.StatusOK, logoutStatus)

	var logoutResp authLogoutResponse
	s.requireJSON(logoutBody, &logoutResp)
	s.Require().Equal("logged out", logoutResp.Message)

	refreshAfterLogoutStatus, refreshAfterLogoutBody, _ := s.postJSONWithClient(headerClient, "/auth/refresh", nil, bearerHeaders(activeRefreshToken))
	s.Require().Equal(http.StatusUnauthorized, refreshAfterLogoutStatus)

	var refreshAfterLogoutErr authErrorResponse
	s.requireJSON(refreshAfterLogoutBody, &refreshAfterLogoutErr)
	s.Require().NotEmpty(refreshAfterLogoutErr.Error)
	s.Require().Contains(strings.ToLower(refreshAfterLogoutErr.Error), "refresh token")
}

func (s *E2ESuite) postJSON(path string, body any, headers map[string]string) (int, []byte) {
	status, respBody, _ := s.postJSONWithClient(s.client, path, body, headers)
	return status, respBody
}

func (s *E2ESuite) postJSONWithClient(client *http.Client, path string, body any, headers map[string]string) (int, []byte, []*http.Cookie) {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		s.Require().NoError(err)
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(http.MethodPost, s.baseURL+path, bodyReader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	return resp.StatusCode, respBody, resp.Cookies()
}

func bearerHeaders(refreshToken string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + refreshToken}
}

func nextRefreshToken(current string, refreshResp authRefreshResponse, cookies []*http.Cookie) string {
	if refreshResp.RefreshToken != "" {
		return refreshResp.RefreshToken
	}
	for _, cookie := range cookies {
		if cookie.Name == "refresh_token" && cookie.Value != "" {
			return cookie.Value
		}
	}
	return current
}

func (s *E2ESuite) requireJSON(data []byte, dst any) {
	err := json.Unmarshal(data, dst)
	s.Require().NoError(err, "body: %s", string(data))
}
