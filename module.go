package sitemanager

import (
	"webtyp.com/ddl"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/router"
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

func (m *Module) MountOps(reg router.OpRegistry) {
	reg.Op("site_get", func(ctx router.Context) {
		userID := ctx.UserID()
		if userID == "" {
			ctx.WriteStatus(401)
			return
		}

		var s Site
		if err := ctx.Decode(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		if s.Id == "" {
			ctx.WriteStatus(400)
			return
		}

		if _, ok := m.MemberOf(userID, s.Id); !ok {
			ctx.WriteStatus(403)
			return
		}

		if err := m.db.Query(&s).Where(Site_.Id).Eq(s.Id).ReadOne(); err != nil {
			ctx.WriteStatus(404)
			return
		}
		_ = ctx.Encode(&s)
	}).Requires(model.Resource("site"), model.Read).Accepts(&Site{})

	reg.Op("site_create", func(ctx router.Context) {
		userID := ctx.UserID()
		if userID == "" {
			ctx.WriteStatus(401)
			return
		}

		var s Site
		if err := ctx.Decode(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		if err := m.CreateSite(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}

		if _, err := m.AddMember(s.Id, userID, RoleOwner); err != nil {
			ctx.WriteStatus(500)
			_ = ctx.Encode(&s)
			return
		}

		ctx.WriteStatus(201)
		_ = ctx.Encode(&s)
	}).Requires(model.Resource("site"), model.Create).Accepts(&Site{})

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
	}).Authenticated().Accepts(&AccessRequest{})
}
