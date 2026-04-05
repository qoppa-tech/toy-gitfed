package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	maxNameLen     = 255
	maxUsernameLen = 255
	maxEmailLen    = 255
	maxPasswordLen = 72  // bcrypt hard limit
	minPasswordLen = 8
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/register", h.Register)
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

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
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

	user, err := h.service.Register(r.Context(), RegisterInput{
		Name:     req.Name,
		Username: req.Username,
		Password: req.Password,
		Email:    req.Email,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already taken"})
		case errors.Is(err, ErrUsernameTaken):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "username already taken"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	writeJSON(w, http.StatusCreated, userResponse{
		UserID:   uuidString(user.UserID),
		Name:     user.Name,
		Username: user.Username,
		Email:    user.Email,
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	id, _ := uuid.FromBytes(u.Bytes[:])
	return id.String()
}
