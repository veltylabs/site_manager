package sitemanager

import (
	"webtyp.com/ddl"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/router"
)

var _ router.OperationModule = (*Module)(nil)

type Deps struct {
	DB  *orm.DB
	IDs model.IDGenerator
}

type Module struct {
	db  *orm.DB
	ids model.IDGenerator
}

// New inicializa el módulo.
//
// Ejemplo: sm, err := sitemanager.New(deps)
func New(d Deps) (*Module, error) {
	if d.DB == nil || d.IDs == nil {
		return nil, ErrNilDependency
	}

	m := &Module{
		db:  d.DB,
		ids: d.IDs,
	}

	if c, ok := d.DB.RawConn().(ddl.Compiler); ok {
		if err := Migrate(d.DB.RawConn(), c); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// ModelName devuelve el nombre del modelo.
//
// Ejemplo: name := m.ModelName()
func (m *Module) ModelName() string {
	return "site"
}

// MountOperations registra las operaciones en el enrutador.
//
// Ejemplo: sm.MountOperations(reg)
func (m *Module) MountOperations(reg router.OperationRegistry) {
	reg.Operation("site_get", func(ctx router.Context) {
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

	reg.Operation("site_create", func(ctx router.Context) {
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

	reg.Operation("access_request", func(ctx router.Context) {
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
