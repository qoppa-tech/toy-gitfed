package tls

import "errors"

var (
	ErrCertNotFound  = errors.New("tls: certificate or key file not found")
	ErrCertExpired   = errors.New("tls: certificate has expired")
	ErrCertInvalid   = errors.New("tls: certificate is invalid")
	ErrACMEChallenge = errors.New("tls: ACME challenge failed")
)
