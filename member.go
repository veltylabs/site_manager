package sitemanager

type Role uint8

const (
	RoleViewer Role = iota // ZERO VALUE — el permiso más bajo
	RoleEditor
	RoleOwner
)

// String devuelve la representación en texto del rol.
//
// Ejemplo: str := RoleOwner.String()
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

// MemberOf responde si un usuario puede tocar un sitio y con qué rol.
// Es la ÚNICA manera de responder esta pregunta.
//
// Ejemplo: role, ok := m.MemberOf("usr_123", "site_456")
func (m *Module) MemberOf(userID, siteID string) (Role, bool) {
	if userID == "" || siteID == "" {
		return RoleViewer, false
	}
	var member SiteMember
	err := m.db.Query(&member).
		Where(SiteMember_.UserId).Eq(userID).
		Where(SiteMember_.SiteId).Eq(siteID).
		ReadOne()
	if err != nil {
		return RoleViewer, false
	}
	return Role(member.Role), true
}

// SitesOf devuelve los sitios donde el usuario tiene membresía.
// Puede devolver un slice vacío: esto es estado normal, no un error.
//
// Ejemplo: sites, err := m.SitesOf("usr_123")
func (m *Module) SitesOf(userID string) ([]Site, error) {
	if userID == "" {
		return []Site{}, nil
	}
	members, err := ReadAllSiteMember(
		m.db.Query(&SiteMember{}).Where(SiteMember_.UserId).Eq(userID),
	)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return []Site{}, nil
	}

	res := make([]Site, 0, len(members))
	for _, mem := range members {
		var s Site
		err := m.db.Query(&s).Where(Site_.Id).Eq(mem.SiteId).ReadOne()
		if err == nil {
			res = append(res, s)
		}
	}
	return res, nil
}

// AddMember añade la membresía de un usuario a un sitio.
//
// Ejemplo: member, err := m.AddMember("site_456", "usr_123", RoleOwner)
func (m *Module) AddMember(siteID, userID string, role Role) (*SiteMember, error) {
	if siteID == "" || userID == "" {
		return nil, ErrInvalidData
	}
	mem := &SiteMember{
		Id:     m.ids.NewID(),
		SiteId: siteID,
		UserId: userID,
		Role:   int64(role),
	}
	if err := m.db.Create(mem); err != nil {
		return nil, err
	}
	return mem, nil
}
