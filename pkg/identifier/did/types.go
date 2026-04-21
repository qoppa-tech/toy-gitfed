package did

// Method identifies the DID method.
type Method string

const (
	MethodGitFed Method = "gitfed"
)

// PrincipalType identifies which first-class principal the DID represents.
// v1 only supports user and server. Repositories stay node references.
type PrincipalType string

const (
	PrincipalTypeUser   PrincipalType = "user"
	PrincipalTypeServer PrincipalType = "server"
)

// NodeKind represents node-level entities in the forge graph.
type NodeKind string

const (
	NodeKindServer       NodeKind = "server"
	NodeKindUser         NodeKind = "user"
	NodeKindRepository   NodeKind = "repository"
	NodeKindOrganization NodeKind = "organization"
)

// NodeRef is an abstraction for non-DID subnodes (repo/org/etc) in the node view.
type NodeRef struct {
	Kind      NodeKind `json:"kind"`
	ID        string   `json:"id"`
	ParentDID string   `json:"parent_did,omitempty"`
}

// DID is a parsed decentralized identifier.
type DID struct {
	Method        Method
	PrincipalType PrincipalType
	Identifier    string
	Host          string
}
