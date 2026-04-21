package did

import (
	"strings"

	apperrors "github.com/qoppa-tech/toy-gitfed/pkg/errors"
)

// Parse parses and validates a DID string.
// Supported canonical forms:
//   - did:gitfed:user:<username>@<host>
//   - did:gitfed:server:<host>
func Parse(raw string) (DID, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 4 || parts[0] != "did" {
		return DID{}, apperrors.New(apperrors.CodeInvalidArgument, "invalid did format")
	}

	d := DID{
		Method:        Method(strings.ToLower(strings.TrimSpace(parts[1]))),
		PrincipalType: PrincipalType(strings.ToLower(strings.TrimSpace(parts[2]))),
	}

	tail := strings.TrimSpace(parts[3])
	switch d.PrincipalType {
	case PrincipalTypeUser:
		identifier, host, ok := strings.Cut(tail, "@")
		if !ok || identifier == "" || host == "" {
			return DID{}, apperrors.New(apperrors.CodeInvalidArgument, "invalid user did format")
		}
		d.Identifier = strings.ToLower(strings.TrimSpace(identifier))
		d.Host = strings.ToLower(strings.TrimSpace(host))
	case PrincipalTypeServer:
		d.Host = strings.ToLower(strings.TrimSpace(tail))
	default:
		return DID{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported did principal type")
	}

	if err := Validate(d); err != nil {
		return DID{}, err
	}
	return d, nil
}
