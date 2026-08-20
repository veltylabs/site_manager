package sitemanager

type Role uint8

const (
	RoleViewer Role = iota // ZERO VALUE — el permiso más bajo
	RoleEditor
	RoleOwner
)

func (r Role) String() string {
	switch r {
	case RoleViewer:
		return "viewer"
	case RoleEditor:
		return "editor"
	case RoleOwner:
		return "owner"
	default:
		return "unknown"
	}
}
