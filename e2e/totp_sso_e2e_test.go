package e2e

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
)

type totpSetupResponse struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

type totpVerifyResponse struct {
	Message string `json:"message"`
}

func (s *E2ESuite) TestTOTPSetupAndVerifyFlow() {
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	password := "Passw0rd!"

	registerReq := authRegisterRequest{
		Name:     "E2E TOTP " + suffix,
		Username: "e2e_totp_" + suffix,
		Password: password,
		Email:    "e2e-totp-" + suffix + "@example.com",
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

	setupStatus, setupBody := s.postJSON("/auth/totp/setup", nil, bearerHeaders(loginResp.AccessToken))
	s.Require().Equal(http.StatusOK, setupStatus)

	var setupResp totpSetupResponse
	s.requireJSON(setupBody, &setupResp)
	s.Require().NotEmpty(setupResp.Secret)
	s.Require().NotEmpty(setupResp.URL)

	parsedTOTPURL, err := url.Parse(setupResp.URL)
	s.Require().NoError(err)
	s.Require().Equal(setupResp.Secret, parsedTOTPURL.Query().Get("secret"))

	verifyStatus := 0
	var verifyBody []byte
	for range 3 {
		code, genErr := totp.GenerateCode(setupResp.Secret, time.Now())
		s.Require().NoError(genErr)

		verifyStatus, verifyBody = s.postJSON("/auth/totp/verify", map[string]string{"code": code}, bearerHeaders(loginResp.AccessToken))
		if verifyStatus == http.StatusOK {
			break
		}

		time.Sleep(1 * time.Second)
	}
	s.Require().Equal(http.StatusOK, verifyStatus)

	var verifyResp totpVerifyResponse
	s.requireJSON(verifyBody, &verifyResp)
	s.Require().Equal("totp verified", verifyResp.Message)
}

func (s *E2ESuite) TestSSOCallbackMissingParamsReturnsBadRequest() {
	status, body := s.get("/auth/google/callback", nil, s.client)
	s.Require().Equal(http.StatusBadRequest, status)

	var errResp authErrorResponse
	s.requireJSON(body, &errResp)
	s.Require().Contains(strings.ToLower(errResp.Error), "missing")
}

func (s *E2ESuite) TestSSORedirectRouteExistsAndTemporaryRedirect() {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest(http.MethodGet, s.baseURL+"/auth/google", nil)
	s.Require().NoError(err)

	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusTemporaryRedirect, resp.StatusCode)

	location := resp.Header.Get("Location")
	s.Require().NotEmpty(location)

	redirectURL, err := url.Parse(location)
	s.Require().NoError(err)
	s.Require().Equal("https", redirectURL.Scheme)
	s.Require().Contains(redirectURL.Host, "google")
	s.Require().True(redirectURL.Query().Has("client_id"))
	s.Require().NotEmpty(redirectURL.Query().Get("redirect_uri"))
	s.Require().NotEmpty(redirectURL.Query().Get("state"))
}

func (s *E2ESuite) get(path string, headers map[string]string, client *http.Client) (int, []byte) {
	req, err := http.NewRequest(http.MethodGet, s.baseURL+path, nil)
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
