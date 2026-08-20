package sitemanager

import (
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/time"
)

var _ router.OpModule = (*Module)(nil)

type Deps struct {
	DB  *orm.DB
	IDs model.IDGenerator
}

type Module struct {
	db  *orm.DB
	ids model.IDGenerator
}

func New(d Deps) (*Module, error) {
	if d.DB == nil || d.IDs == nil {
		return nil, ErrNilDependency
	}

	m := &Module{
		db:  d.DB,
		ids: d.IDs,
	}

	if ddlCompiler, ok := d.DB.RawConn().(ddl.Compiler); ok {
		compiler := ddl.New(d.DB.RawConn(), ddlCompiler)
		if err := compiler.CreateTable(&Site{}); err != nil {
			return nil, err
		}
		if err := compiler.CreateTable(&SiteMember{}); err != nil {
			return nil, err
		}
		if err := compiler.CreateTable(&Plan{}); err != nil {
			return nil, err
		}
		if err := compiler.CreateTable(&AccessRequest{}); err != nil {
			return nil, err
		}
	}

	return m, nil
}

func (m *Module) ModelName() string {
	return "site"
}

// MemberOf responds whether a user can touch a site, and with what role.
// It is the ONLY way to answer this question.
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

// SitesOf returns the sites where the user has membership.
// Can return empty slice: normal state, not an error.
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

// Request registers an access request. If a pending request already exists for that email,
// returns the existing one and created=false.
func (m *Module) Request(email, name, message string) (req AccessRequest, created bool, err error) {
	if email == "" {
		return AccessRequest{}, false, ErrInvalidData
	}

	pending, err := ReadAllAccessRequest(
		m.db.Query(&AccessRequest{}).
			Where(AccessRequest_.Email).Eq(email).
			Where(AccessRequest_.Status).Eq(int64(RequestPending)),
	)
	if err == nil && len(pending) > 0 {
		return *pending[0], false, nil
	}

	nowSec := time.Now() / 1_000_000_000
	newID := m.ids.NewID()
	req = AccessRequest{
		Id:        newID,
		Email:     email,
		Name:      name,
		Message:   message,
		CreatedAt: nowSec,
		Status:    int64(RequestPending),
	}

	if err := m.db.Create(&req); err != nil {
		return AccessRequest{}, false, err
	}

	return req, true, nil
}

// AcceptRequest accepts an access request by setting its status to RequestAccepted.
// Note: Accepting an AccessRequest creates ZERO SiteMember entries.
func (m *Module) AcceptRequest(id string) error {
	var req AccessRequest
	err := m.db.Query(&req).Where(AccessRequest_.Id).Eq(id).ReadOne()
	if err != nil {
		return err
	}
	req.Status = int64(RequestAccepted)
	return m.db.Update(&req, orm.Eq(AccessRequest_.Id, id))
}

// CreateSite creates a new site ensuring slug uniqueness.
func (m *Module) CreateSite(site *Site) error {
	if site == nil {
		return ErrInvalidData
	}
	if site.Slug == "" || site.Name == "" || site.Theme == "" {
		return ErrInvalidData
	}

	// Check unique slug constraint
	var existing Site
	err := m.db.Query(&existing).Where(Site_.Slug).Eq(site.Slug).ReadOne()
	if err == nil {
		return ErrAlreadyExists
	}

	if site.Id == "" {
		site.Id = m.ids.NewID()
	}

	return m.db.Create(site)
}

// UpdateSiteStatus updates a site's status following valid state transitions.
func (m *Module) UpdateSiteStatus(siteID string, newStatus Status) error {
	var site Site
	err := m.db.Query(&site).Where(Site_.Id).Eq(siteID).ReadOne()
	if err != nil {
		return err
	}

	currentStatus := Status(site.Status)
	if err := ValidateTransition(currentStatus, newStatus); err != nil {
		return err
	}

	site.Status = int64(newStatus)
	return m.db.Update(&site, orm.Eq(Site_.Id, siteID))
}

// AddMember adds a user membership to a site.
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

func (m *Module) MountOps(reg router.OpRegistry) {
	reg.Op("site_get", func(ctx router.Context) {
		var s Site
		if err := ctx.Decode(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		if err := m.db.Query(&s).Where(Site_.Id).Eq(s.Id).ReadOne(); err != nil {
			ctx.WriteStatus(404)
			return
		}
		_ = ctx.Encode(&s)
	}).Requires(model.Resource("site"), model.Read)

	reg.Op("site_create", func(ctx router.Context) {
		var s Site
		if err := ctx.Decode(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		if err := m.CreateSite(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		_ = ctx.Encode(&s)
	}).Requires(model.Resource("site"), model.Create)

	reg.Op("access_request", func(ctx router.Context) {
		var req AccessRequest
		if err := ctx.Decode(&req); err != nil {
			ctx.WriteStatus(400)
			return
		}
		createdReq, _, err := m.Request(req.Email, req.Name, req.Message)
		if err != nil {
			ctx.WriteStatus(400)
			return
		}
		_ = ctx.Encode(&createdReq)
	}).Public()
}
