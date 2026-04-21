package did

import "fmt"

// String returns canonical DID representation.
func (d DID) String() string {
	switch d.PrincipalType {
	case PrincipalTypeUser:
		return fmt.Sprintf("did:%s:%s:%s@%s", d.Method, d.PrincipalType, d.Identifier, d.Host)
	case PrincipalTypeServer:
		return fmt.Sprintf("did:%s:%s:%s", d.Method, d.PrincipalType, d.Host)
	default:
		return fmt.Sprintf("did:%s:%s:%s", d.Method, d.PrincipalType, d.Host)
	}
}
