package user

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("user not found")
	ErrEmailTaken    = errors.New("email already taken")
	ErrUsernameTaken = errors.New("username already taken")
)

type User struct {
	ID         uuid.UUID
	Name       string
	Username   string
	Email      string
	Password   string // bcrypt hash
	CreatedAt  time.Time
	UpdatedAt  time.Time
	IsDeleted  bool
	IsVerified bool
}

type RegisterInput struct {
	Name     string
	Username string
	Password string // plaintext — hashed by service
	Email    string
}
