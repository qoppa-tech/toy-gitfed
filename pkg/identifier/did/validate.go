package did

import (
	"regexp"
	"strings"

	apperrors "github.com/qoppa-tech/toy-gitfed/pkg/errors"
)

var (
	usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
	hostPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,252}[a-z0-9]$`)
)

// Validate validates DID invariants.
func Validate(d DID) error {
	if d.Method != MethodGitFed {
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported did method")
	}

	switch d.PrincipalType {
	case PrincipalTypeUser:
		if d.Identifier == "" {
			return apperrors.New(apperrors.CodeInvalidArgument, "user did identifier is required")
		}
		if !usernamePattern.MatchString(d.Identifier) {
			return apperrors.New(apperrors.CodeInvalidArgument, "invalid user did identifier")
		}
	case PrincipalTypeServer:
		if d.Identifier != "" {
			return apperrors.New(apperrors.CodeInvalidArgument, "server did must not contain identifier")
		}
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported did principal type")
	}

	if d.Host == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "did host is required")
	}
	if strings.Contains(d.Host, "..") || strings.HasPrefix(d.Host, ".") || strings.HasSuffix(d.Host, ".") {
		return apperrors.New(apperrors.CodeInvalidArgument, "invalid did host")
	}
	if !hostPattern.MatchString(d.Host) {
		return apperrors.New(apperrors.CodeInvalidArgument, "invalid did host")
	}

	return nil
}
