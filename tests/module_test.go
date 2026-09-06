package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/orm"
	mockrouter "webtyp.com/router/mock"
	"webtyp.com/storage/mem"
	sitemanager "github.com/veltylabs/site_manager"
)

// -- Status transitions ------------------------------------------------------

// Mismo estado no es una transición listada en la tabla: debe fallar, no
// tratarse como no-op. Un doble click en el panel se evita deshabilitando el
// boton en la UI, no relajando la regla de dominio.
func TestValidateTransitionSameStateIsInvalid(t *testing.T) {
	cases := []sitemanager.Status{
		sitemanager.StatusDraft,
		sitemanager.StatusLive,
		sitemanager.StatusSuspended,
	}
	for _, s := range cases {
		if err := sitemanager.ValidateTransition(s, s); err == nil {
			t.Fatalf("expected error for %v -> %v (same state), got nil", s, s)
		}
	}
}

// -- New() dependency validation ---------------------------------------------

func TestNewRequiresDB(t *testing.T) {
	_, err := sitemanager.New(sitemanager.Deps{IDs: &testIDGenerator{}})
	if err == nil {
		t.Fatal("expected error when DB is nil, got nil")
	}
}

func TestNewRequiresIDs(t *testing.T) {
	db := orm.New(mem.New())
	_, err := sitemanager.New(sitemanager.Deps{DB: db})
	if err == nil {
		t.Fatal("expected error when IDs is nil, got nil")
	}
}

// -- CreateSite / AddMember / Request guard clauses ---------------------------

func TestCreateSiteRejectsInvalidData(t *testing.T) {
	m, _ := setupModule(t)

	if err := m.CreateSite(nil); err == nil {
		t.Fatal("expected error for nil site, got nil")
	}
	if err := m.CreateSite(&sitemanager.Site{Name: "x", Theme: "landing"}); err == nil {
		t.Fatal("expected error for empty slug, got nil")
	}
	if err := m.CreateSite(&sitemanager.Site{Slug: "x", Theme: "landing"}); err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if err := m.CreateSite(&sitemanager.Site{Slug: "x", Name: "x"}); err == nil {
		t.Fatal("expected error for empty theme, got nil")
	}
}

func TestAddMemberRejectsEmptyIDs(t *testing.T) {
	m, _ := setupModule(t)

	if _, err := m.AddMember("", "user1", sitemanager.RoleViewer); err == nil {
		t.Fatal("expected error for empty siteID, got nil")
	}
	if _, err := m.AddMember("site1", "", sitemanager.RoleViewer); err == nil {
		t.Fatal("expected error for empty userID, got nil")
	}
}

func TestRequestRejectsEmptyEmail(t *testing.T) {
	m, _ := setupModule(t)

	if _, _, err := m.Request("", "Name", "msg"); err == nil {
		t.Fatal("expected error for empty email, got nil")
	}
}

func TestMemberOfRejectsEmptyIDs(t *testing.T) {
	m, _ := setupModule(t)

	if role, ok := m.MemberOf("", "site1"); ok || role != sitemanager.RoleViewer {
		t.Fatalf("expected (RoleViewer, false) for empty userID, got (%v, %v)", role, ok)
	}
	if role, ok := m.MemberOf("user1", ""); ok || role != sitemanager.RoleViewer {
		t.Fatalf("expected (RoleViewer, false) for empty siteID, got (%v, %v)", role, ok)
	}
}

// -- AcceptRequest / UpdateSiteStatus error paths -----------------------------

func TestAcceptRequestNotFound(t *testing.T) {
	m, _ := setupModule(t)

	if err := m.AcceptRequest("does-not-exist"); err == nil {
		t.Fatal("expected error accepting a nonexistent request, got nil")
	}
}

func TestUpdateSiteStatusNotFound(t *testing.T) {
	m, _ := setupModule(t)

	if err := m.UpdateSiteStatus("does-not-exist", sitemanager.StatusLive); err == nil {
		t.Fatal("expected error updating a nonexistent site, got nil")
	}
}

func TestUpdateSiteStatusInvalidTransitionLeavesSiteUnchanged(t *testing.T) {
	m, db := setupModule(t)

	site := &sitemanager.Site{Slug: "site-x", Name: "Site X", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}

	err := m.UpdateSiteStatus(site.Id, sitemanager.StatusSuspended)
	if err == nil {
		t.Fatal("expected error for draft -> suspended, got nil")
	}

	var reloaded sitemanager.Site
	if err := db.Query(&reloaded).Where(sitemanager.Site_.Id).Eq(site.Id).ReadOne(); err != nil {
		t.Fatalf("reload site: %v", err)
	}
	if sitemanager.Status(reloaded.Status) != sitemanager.StatusDraft {
		t.Fatalf("expected status to remain draft after rejected transition, got %v", reloaded.Status)
	}
}

func TestUpdateSiteStatusValidTransitionPersists(t *testing.T) {
	m, db := setupModule(t)

	site := &sitemanager.Site{Slug: "site-y", Name: "Site Y", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}

	if err := m.UpdateSiteStatus(site.Id, sitemanager.StatusLive); err != nil {
		t.Fatalf("expected draft -> live to succeed, got %v", err)
	}

	var reloaded sitemanager.Site
	if err := db.Query(&reloaded).Where(sitemanager.Site_.Id).Eq(site.Id).ReadOne(); err != nil {
		t.Fatalf("reload site: %v", err)
	}
	if sitemanager.Status(reloaded.Status) != sitemanager.StatusLive {
		t.Fatalf("expected persisted status live, got %v", reloaded.Status)
	}
}

// -- MountOps / router.OpModule contract --------------------------------------

func setupRouter(t *testing.T, authorize model.Authorizer) (*sitemanager.Module, *mockrouter.Router) {
	m, _ := setupModule(t)
	reg := &mockrouter.Router{}
	reg.Configure(mockrouter.Config{Authorize: authorize})
	m.MountOps(reg)
	return m, reg
}

// site_create esta anotado Requires(site, create): sin identidad ni Authorize
// configurado debe denegar, no ejecutar la logica de negocio.
func TestMountOpsSiteCreateDeniesWithoutAuthorization(t *testing.T) {
	_, reg := setupRouter(t, nil)

	ctx := &mockrouter.Context{
		InBody: []byte(`{"slug":"denied","name":"Denied","theme":"landing"}`),
	}
	reg.Invoke("OP", "/site_create", ctx)

	if ctx.Status != 403 {
		t.Fatalf("expected 403 for unauthorized site_create, got %d", ctx.Status)
	}
}

func TestMountOpsSiteCreateAllowsAuthorizedCaller(t *testing.T) {
	m, reg := setupRouter(t, func(userID string, r model.Resource, a model.Action) bool {
		return userID == "owner1" && r == model.Resource("site") && a == model.Create
	})

	ctx := &mockrouter.Context{
		InBody: []byte(`{"slug":"allowed","name":"Allowed","theme":"landing"}`),
	}
	ctx.SetUserID("owner1")
	reg.Invoke("OP", "/site_create", ctx)

	if ctx.Status != 201 {
		t.Fatalf("expected authorized site_create to succeed with status 201, got status %d body %s", ctx.Status, ctx.ResponseBody())
	}
	if len(ctx.ResponseBody()) == 0 {
		t.Fatal("expected a non-empty response body for created site")
	}

	sites, err := m.SitesOf("owner1")
	if err != nil {
		t.Fatalf("SitesOf: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("expected site_create to add caller as member, got %d sites for owner1", len(sites))
	}
}

func TestSiteGetDeniesNonMember(t *testing.T) {
	m, reg := setupRouter(t, func(userID string, r model.Resource, a model.Action) bool {
		return r == model.Resource("site") && a == model.Read
	})

	site := &sitemanager.Site{Slug: "site1", Name: "Site 1", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite: %v", err)
	}

	ctx := &mockrouter.Context{
		InBody: []byte(`{"id":"` + site.Id + `"}`),
	}
	ctx.SetUserID("nonmember")
	reg.Invoke("OP", "/site_get", ctx)

	if ctx.Status != 403 {
		t.Fatalf("expected 403 for non-member site_get, got %d", ctx.Status)
	}
}

func TestSiteGetAllowsMember(t *testing.T) {
	m, reg := setupRouter(t, func(userID string, r model.Resource, a model.Action) bool {
		return r == model.Resource("site") && a == model.Read
	})

	site := &sitemanager.Site{Slug: "site2", Name: "Site 2", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite: %v", err)
	}

	if _, err := m.AddMember(site.Id, "member1", sitemanager.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	ctx := &mockrouter.Context{
		InBody: []byte(`{"id":"` + site.Id + `"}`),
	}
	ctx.SetUserID("member1")
	reg.Invoke("OP", "/site_get", ctx)

	if ctx.Status != 200 && ctx.Status != 0 {
		t.Fatalf("expected 200 (or implicit 0 from WriteStatus not being called) for member site_get, got %d", ctx.Status)
	}
	if len(ctx.ResponseBody()) == 0 {
		t.Fatal("expected non-empty response body for member site_get")
	}
}

func TestSiteGetRejectsAnonymous(t *testing.T) {
	_, reg := setupRouter(t, nil)

	ctx := &mockrouter.Context{
		InBody: []byte(`{"id":"site-id"}`),
	}
	reg.Invoke("OP", "/site_get", ctx)

	if ctx.Status != 401 && ctx.Status != 403 {
		t.Fatalf("expected 401 or 403 for anonymous site_get, got %d", ctx.Status)
	}
}

func TestSiteGetRejectsEmptyID(t *testing.T) {
	_, reg := setupRouter(t, func(userID string, r model.Resource, a model.Action) bool {
		return true
	})

	ctx := &mockrouter.Context{
		InBody: []byte(`{"id":""}`),
	}
	ctx.SetUserID("user1")
	reg.Invoke("OP", "/site_get", ctx)

	if ctx.Status != 400 {
		t.Fatalf("expected 400 for empty id site_get, got %d", ctx.Status)
	}
}

func TestSiteCreateAddsOwner(t *testing.T) {
	m, reg := setupRouter(t, func(userID string, r model.Resource, a model.Action) bool {
		return true
	})

	ctx := &mockrouter.Context{
		InBody: []byte(`{"slug":"owner-test","name":"Owner Test","theme":"landing"}`),
	}
	ctx.SetUserID("creator1")
	reg.Invoke("OP", "/site_create", ctx)

	if ctx.Status != 201 {
		t.Fatalf("expected 201 for site_create, got %d", ctx.Status)
	}

	sites, err := m.SitesOf("creator1")
	if err != nil {
		t.Fatalf("SitesOf: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("expected 1 site for creator1, got %d", len(sites))
	}
	role, ok := m.MemberOf("creator1", sites[0].Id)
	if !ok || role != sitemanager.RoleOwner {
		t.Fatalf("expected RoleOwner for site creator, got role %v, ok %v", role, ok)
	}
}

func TestSiteCreateReturns201(t *testing.T) {
	_, reg := setupRouter(t, func(userID string, r model.Resource, a model.Action) bool {
		return true
	})

	ctx := &mockrouter.Context{
		InBody: []byte(`{"slug":"return-201","name":"Return 201","theme":"landing"}`),
	}
	ctx.SetUserID("creator2")
	reg.Invoke("OP", "/site_create", ctx)

	if ctx.Status != 201 {
		t.Fatalf("expected status 201 for site_create, got %d", ctx.Status)
	}
}

func TestAccessRequestRejectsAnonymous(t *testing.T) {
	_, reg := setupRouter(t, nil)

	ctx := &mockrouter.Context{
		InBody: []byte(`{"email":"prospect@example.com","name":"Prospect","message":"hola"}`),
	}
	reg.Invoke("OP", "/access_request", ctx)

	if ctx.Status != 403 {
		t.Fatalf("expected 403 for anonymous access_request, got %d", ctx.Status)
	}
}

func TestAccessRequestAllowsAuthenticated(t *testing.T) {
	_, reg := setupRouter(t, func(userID string, r model.Resource, a model.Action) bool {
		return false // No resource roles granted
	})

	ctx := &mockrouter.Context{
		InBody: []byte(`{"email":"prospect@example.com","name":"Prospect","message":"hola"}`),
	}
	ctx.SetUserID("auth_user")
	reg.Invoke("OP", "/access_request", ctx)

	if ctx.Status == 403 || ctx.Status == 401 {
		t.Fatalf("expected authenticated access_request to succeed, got status %d", ctx.Status)
	}
	if len(ctx.ResponseBody()) == 0 {
		t.Fatal("expected non-empty response body for access_request")
	}
}

func TestOpsDeclareArgs(t *testing.T) {
	_, reg := setupRouter(t, nil)

	routes := reg.Routes()
	if len(routes) == 0 {
		t.Fatal("expected registered routes, got 0")
	}

	for _, route := range routes {
		if route.Args == nil {
			t.Fatalf("route %s has nil Args", route.Path)
		}
	}
}
