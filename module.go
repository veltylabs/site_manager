package sitemanager

import (
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
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
		var s Site
		if err := router.Decode(ctx, &s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		if err := m.db.Query(&s).Where(Site_.Id).Eq(s.Id).ReadOne(); err != nil {
			ctx.WriteStatus(404)
			return
		}
		_ = router.Encode(ctx, &s)
	}).Requires(model.Resource("site"), model.Read)

	reg.Op("site_create", func(ctx router.Context) {
		var s Site
		if err := router.Decode(ctx, &s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		if err := m.CreateSite(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		_ = router.Encode(ctx, &s)
	}).Requires(model.Resource("site"), model.Create)

	reg.Op("access_request", func(ctx router.Context) {
		var req AccessRequest
		if err := router.Decode(ctx, &req); err != nil {
			ctx.WriteStatus(400)
			return
		}
		createdReq, _, err := m.Request(req.Email, req.Name, req.Message)
		if err != nil {
			ctx.WriteStatus(400)
			return
		}
		_ = router.Encode(ctx, &createdReq)
	}).Public()
}
