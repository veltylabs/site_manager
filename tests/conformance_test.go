package tests

import (
	"testing"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/storage/mem"
	sitemanager "github.com/veltylabs/site_manager"
)

type testIDGenerator struct {
	count int
}

func (g *testIDGenerator) NewID() string {
	g.count++
	return fmt.Sprintf("id-%d", g.count)
}

func setupModule(t *testing.T) (*sitemanager.Module, *orm.DB) {
	db := orm.New(mem.New())
	ids := &testIDGenerator{}
	m, err := sitemanager.New(sitemanager.Deps{
		DB:  db,
		IDs: ids,
	})
	if err != nil {
		t.Fatalf("setupModule failed: %v", err)
	}
	return m, db
}

// Case 1: Site recién creado -> Status == StatusDraft sin escribir el campo
func TestSiteDefaultStatus(t *testing.T) {
	s := sitemanager.Site{}
	if sitemanager.Status(s.Status) != sitemanager.StatusDraft {
		t.Fatalf("expected StatusDraft (0), got %v", s.Status)
	}
}

// Case 2: SiteMember recién creado -> Role == RoleViewer sin escribir el campo
func TestSiteMemberDefaultRole(t *testing.T) {
	m := sitemanager.SiteMember{}
	if sitemanager.Role(m.Role) != sitemanager.RoleViewer {
		t.Fatalf("expected RoleViewer (0), got %v", m.Role)
	}
}

// Case 3: AccessRequest recién creada -> Status == RequestPending sin escribir el campo
func TestAccessRequestDefaultStatus(t *testing.T) {
	req := sitemanager.AccessRequest{}
	if sitemanager.RequestStatus(req.Status) != sitemanager.RequestPending {
		t.Fatalf("expected RequestPending (0), got %v", req.Status)
	}
}

// Case 4: Transición draft -> suspended -> error con mensaje textual
func TestInvalidStatusTransition(t *testing.T) {
	err := sitemanager.ValidateTransition(sitemanager.StatusDraft, sitemanager.StatusSuspended)
	if err == nil {
		t.Fatal("expected transition error for draft -> suspended, got nil")
	}
	expectedMsg := "site_manager: transicion invalida de draft a suspended"
	if err.Error() != expectedMsg {
		t.Fatalf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// Case 5: Transición draft -> live -> ok
func TestValidStatusTransition(t *testing.T) {
	err := sitemanager.ValidateTransition(sitemanager.StatusDraft, sitemanager.StatusLive)
	if err != nil {
		t.Fatalf("expected valid transition draft -> live, got %v", err)
	}
}

// Case 6: MemberOf de un usuario sin membresía -> (RoleViewer, false)
func TestMemberOfNonMember(t *testing.T) {
	m, _ := setupModule(t)
	role, ok := m.MemberOf("user1", "site1")
	if ok {
		t.Fatalf("expected ok=false for non-member, got ok=true")
	}
	if role != sitemanager.RoleViewer {
		t.Fatalf("expected RoleViewer for non-member, got %v", role)
	}
}

// Case 7: MemberOf de un usuario con membresía -> el rol correcto, true
func TestMemberOfMember(t *testing.T) {
	m, _ := setupModule(t)
	site := &sitemanager.Site{
		Slug:  "mysite",
		Name:  "My Site",
		Theme: "landing",
	}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("failed to create site: %v", err)
	}

	_, err := m.AddMember(site.Id, "user1", sitemanager.RoleEditor)
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	role, ok := m.MemberOf("user1", site.Id)
	if !ok {
		t.Fatalf("expected ok=true for member, got false")
	}
	if role != sitemanager.RoleEditor {
		t.Fatalf("expected RoleEditor, got %v", role)
	}
}

// Case 8: SitesOf de un usuario sin sitios -> slice vacío, sin error
func TestSitesOfUserWithNoSites(t *testing.T) {
	m, _ := setupModule(t)
	sites, err := m.SitesOf("user_no_sites")
	if err != nil {
		t.Fatalf("expected no error for user without sites, got %v", err)
	}
	if len(sites) != 0 {
		t.Fatalf("expected empty slice, got len %d", len(sites))
	}
}

// Case 9: SitesOf no devuelve sitios ajenos
func TestSitesOfIsolation(t *testing.T) {
	m, _ := setupModule(t)

	s1 := &sitemanager.Site{Slug: "site1", Name: "Site 1", Theme: "landing"}
	s2 := &sitemanager.Site{Slug: "site2", Name: "Site 2", Theme: "landing"}
	if err := m.CreateSite(s1); err != nil {
		t.Fatalf("create site1: %v", err)
	}
	if err := m.CreateSite(s2); err != nil {
		t.Fatalf("create site2: %v", err)
	}

	if _, err := m.AddMember(s1.Id, "user1", sitemanager.RoleOwner); err != nil {
		t.Fatalf("add member s1: %v", err)
	}
	if _, err := m.AddMember(s2.Id, "user2", sitemanager.RoleOwner); err != nil {
		t.Fatalf("add member s2: %v", err)
	}

	user1Sites, err := m.SitesOf("user1")
	if err != nil {
		t.Fatalf("SitesOf user1: %v", err)
	}
	if len(user1Sites) != 1 || user1Sites[0].Id != s1.Id {
		t.Fatalf("expected only site1 for user1, got %v", user1Sites)
	}
}

// Case 10: Dos Site con el mismo Slug -> error de unicidad
func TestDuplicateSlugError(t *testing.T) {
	m, _ := setupModule(t)

	s1 := &sitemanager.Site{Slug: "duplicate-slug", Name: "Site 1", Theme: "landing"}
	s2 := &sitemanager.Site{Slug: "duplicate-slug", Name: "Site 2", Theme: "landing"}

	if err := m.CreateSite(s1); err != nil {
		t.Fatalf("first CreateSite failed: %v", err)
	}

	err := m.CreateSite(s2)
	if err == nil {
		t.Fatal("expected error on duplicate slug, got nil")
	}
}

// Case 11: Segunda Request con una pendiente del mismo correo -> devuelve la existente, created == false
func TestDuplicatePendingAccessRequest(t *testing.T) {
	m, _ := setupModule(t)

	req1, created1, err := m.Request("test@velty.cl", "Test User", "Need access")
	if err != nil || !created1 {
		t.Fatalf("expected first request created=true, err=nil, got created=%v err=%v", created1, err)
	}

	req2, created2, err := m.Request("test@velty.cl", "Test User", "Need access again")
	if err != nil {
		t.Fatalf("expected second request err=nil, got %v", err)
	}
	if created2 {
		t.Fatalf("expected second request created=false, got created=true")
	}
	if req2.Id != req1.Id {
		t.Fatalf("expected existing request ID %s, got %s", req1.Id, req2.Id)
	}
}

// Case 12: Request tras aceptar la anterior -> una nueva, created == true
func TestRequestAfterAccepted(t *testing.T) {
	m, _ := setupModule(t)

	req1, created1, err := m.Request("test2@velty.cl", "Test User 2", "First request")
	if err != nil || !created1 {
		t.Fatalf("first request failed: created=%v, err=%v", created1, err)
	}

	if err := m.AcceptRequest(req1.Id); err != nil {
		t.Fatalf("AcceptRequest failed: %v", err)
	}

	req2, created2, err := m.Request("test2@velty.cl", "Test User 2", "Second request after accept")
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	if !created2 {
		t.Fatalf("expected created=true for request after previous accepted, got false")
	}
	if req2.Id == req1.Id {
		t.Fatalf("expected new request ID, got same ID %s", req1.Id)
	}
}

// Case 13: Aceptar una AccessRequest -> cero SiteMember creadas
func TestAcceptAccessRequestCreatesNoMembers(t *testing.T) {
	m, db := setupModule(t)

	req, _, err := m.Request("partner@velty.cl", "Partner", "Access please")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if err := m.AcceptRequest(req.Id); err != nil {
		t.Fatalf("AcceptRequest failed: %v", err)
	}

	members, err := sitemanager.ReadAllSiteMember(db.Query(&sitemanager.SiteMember{}))
	if err != nil {
		t.Fatalf("query members failed: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("CRITICAL SECURITY CHECK FAILED: expected 0 SiteMembers after accepting access request, got %d", len(members))
	}
}
