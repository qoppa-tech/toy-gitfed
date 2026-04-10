package sso

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Provider string

const (
	ProviderGoogle    Provider = "google"
	ProviderLocal     Provider = "local"
	ProviderQoppaTech Provider = "qoppatech"
)

var (
	ErrProviderNotFound = errors.New("sso provider record not found")
	ErrInvalidState     = errors.New("invalid oauth state")
)

type Record struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	Provider    Provider
	Username    string
	ActivatedAt time.Time
	CreatedAt   time.Time
}

type GoogleUserInfo struct {
	Sub   string
	Email string
	Name  string
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}
