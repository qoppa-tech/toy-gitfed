package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/qoppa-tech/toy-gitfed/internal/modules/user"
)

const (
	maxNameLen     = 255
	maxUsernameLen = 255
	maxEmailLen    = 255
	maxPasswordLen = 72 // bcrypt hard limit
	minPasswordLen = 8
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type UserPresenter struct {
	service *user.Service
}

func NewUserPresenter(service *user.Service) *UserPresenter {
	return &UserPresenter{service: service}
}

func (p *UserPresenter) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/register", p.Register)
}

type registerRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type userResponse struct {
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (p *UserPresenter) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if req.Name == "" || req.Username == "" || req.Password == "" || req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "all fields are required"})
		return
	}

	if err := validateRegisterInput(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	u, err := p.service.Register(r.Context(), user.RegisterInput{
		Name:     req.Name,
		Username: req.Username,
		Password: req.Password,
		Email:    req.Email,
	})
	if err != nil {
		switch {
		case errors.Is(err, user.ErrEmailTaken):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already taken"})
		case errors.Is(err, user.ErrUsernameTaken):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "username already taken"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	writeJSON(w, http.StatusCreated, userResponse{
		UserID:   u.ID.String(),
		Name:     u.Name,
		Username: u.Username,
		Email:    u.Email,
	})
}

func validateRegisterInput(req registerRequest) error {
	if len(req.Name) > maxNameLen {
		return errors.New("name too long")
	}
	if len(req.Username) > maxUsernameLen {
		return errors.New("username too long")
	}
	if len(req.Email) > maxEmailLen {
		return errors.New("email too long")
	}
	if !emailRegex.MatchString(req.Email) {
		return errors.New("invalid email format")
	}
	if len(req.Password) < minPasswordLen {
		return errors.New("password must be at least 8 characters")
	}
	if len(req.Password) > maxPasswordLen {
		return errors.New("password too long")
	}
	return nil
}
