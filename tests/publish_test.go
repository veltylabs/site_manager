package tests

import (
	"testing"

	sitemanager "github.com/veltylabs/site_manager"
)

// 1. Un sitio limpio queda Dirty == true en la base.
func TestMarkDirtySetsFlag(t *testing.T) {
	m, db := setupModule(t)
	site := &sitemanager.Site{Slug: "site-1", Name: "Site 1", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}

	if err := m.MarkDirty(site.Id); err != nil {
		t.Fatalf("MarkDirty failed: %v", err)
	}

	var reloaded sitemanager.Site
	if err := db.Query(&reloaded).Where(sitemanager.Site_.Id).Eq(site.Id).ReadOne(); err != nil {
		t.Fatalf("ReadOne failed: %v", err)
	}
	if !reloaded.Dirty {
		t.Fatal("expected Dirty == true, got false")
	}
}

// 2. Llamarlo dos veces no falla y el sitio sigue sucio.
func TestMarkDirtyIsIdempotent(t *testing.T) {
	m, db := setupModule(t)
	site := &sitemanager.Site{Slug: "site-2", Name: "Site 2", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}

	if err := m.MarkDirty(site.Id); err != nil {
		t.Fatalf("first MarkDirty failed: %v", err)
	}
	if err := m.MarkDirty(site.Id); err != nil {
		t.Fatalf("second MarkDirty failed: %v", err)
	}

	var reloaded sitemanager.Site
	if err := db.Query(&reloaded).Where(sitemanager.Site_.Id).Eq(site.Id).ReadOne(); err != nil {
		t.Fatalf("ReadOne failed: %v", err)
	}
	if !reloaded.Dirty {
		t.Fatal("expected Dirty == true after second call, got false")
	}
}

// 3. Un sitio Live marcado sucio sigue Live. Es la confusión que originó este plan.
func TestMarkDirtyDoesNotChangeStatus(t *testing.T) {
	m, db := setupModule(t)
	site := &sitemanager.Site{Slug: "site-3", Name: "Site 3", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	if err := m.UpdateSiteStatus(site.Id, sitemanager.StatusLive); err != nil {
		t.Fatalf("UpdateSiteStatus failed: %v", err)
	}

	if err := m.MarkDirty(site.Id); err != nil {
		t.Fatalf("MarkDirty failed: %v", err)
	}

	var reloaded sitemanager.Site
	if err := db.Query(&reloaded).Where(sitemanager.Site_.Id).Eq(site.Id).ReadOne(); err != nil {
		t.Fatalf("ReadOne failed: %v", err)
	}
	if sitemanager.Status(reloaded.Status) != sitemanager.StatusLive {
		t.Fatalf("expected StatusLive, got %v", reloaded.Status)
	}
	if !reloaded.Dirty {
		t.Fatal("expected Dirty == true, got false")
	}
}

// 4. ErrNotFound con un id inexistente; ErrInvalidData con "".
func TestMarkDirtyNotFound(t *testing.T) {
	m, _ := setupModule(t)

	err := m.MarkDirty("")
	if err != sitemanager.ErrInvalidData {
		t.Fatalf("expected ErrInvalidData for empty siteID, got %v", err)
	}

	err = m.MarkDirty("non-existent")
	if err != sitemanager.ErrNotFound {
		t.Fatalf("expected ErrNotFound for non-existent siteID, got %v", err)
	}
}

// 5. Con un sitio sucio y otro limpio, devuelve exactamente uno.
func TestDirtySitesReturnsOnlyDirty(t *testing.T) {
	m, _ := setupModule(t)

	siteClean := &sitemanager.Site{Slug: "clean", Name: "Clean Site", Theme: "landing"}
	siteDirty := &sitemanager.Site{Slug: "dirty", Name: "Dirty Site", Theme: "landing"}
	if err := m.CreateSite(siteClean); err != nil {
		t.Fatalf("CreateSite siteClean failed: %v", err)
	}
	if err := m.CreateSite(siteDirty); err != nil {
		t.Fatalf("CreateSite siteDirty failed: %v", err)
	}

	if err := m.MarkDirty(siteDirty.Id); err != nil {
		t.Fatalf("MarkDirty failed: %v", err)
	}

	dirtySites, err := m.DirtySites()
	if err != nil {
		t.Fatalf("DirtySites failed: %v", err)
	}

	if len(dirtySites) != 1 {
		t.Fatalf("expected 1 dirty site, got %d", len(dirtySites))
	}
	if dirtySites[0].Id != siteDirty.Id {
		t.Fatalf("expected dirty site ID %s, got %s", siteDirty.Id, dirtySites[0].Id)
	}
}

// 6. Un sitio sucio y suspendido no aparece.
func TestDirtySitesExcludesSuspended(t *testing.T) {
	m, _ := setupModule(t)

	site := &sitemanager.Site{Slug: "suspended-dirty", Name: "Suspended Dirty", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	if err := m.UpdateSiteStatus(site.Id, sitemanager.StatusLive); err != nil {
		t.Fatalf("UpdateSiteStatus Live failed: %v", err)
	}
	if err := m.UpdateSiteStatus(site.Id, sitemanager.StatusSuspended); err != nil {
		t.Fatalf("UpdateSiteStatus Suspended failed: %v", err)
	}
	if err := m.MarkDirty(site.Id); err != nil {
		t.Fatalf("MarkDirty failed: %v", err)
	}

	dirtySites, err := m.DirtySites()
	if err != nil {
		t.Fatalf("DirtySites failed: %v", err)
	}

	if len(dirtySites) != 0 {
		t.Fatalf("expected 0 dirty sites (suspended excluded), got %d", len(dirtySites))
	}
}

// 7. Sin sitios sucios: slice vacío y err == nil.
func TestDirtySitesEmptyIsNotAnError(t *testing.T) {
	m, _ := setupModule(t)

	site := &sitemanager.Site{Slug: "clean-only", Name: "Clean Only", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}

	dirtySites, err := m.DirtySites()
	if err != nil {
		t.Fatalf("expected err == nil, got %v", err)
	}
	if len(dirtySites) != 0 {
		t.Fatalf("expected empty slice, got len %d", len(dirtySites))
	}
}

// 8. Dirty == false, PublishedRef guardado, PublishedAt > 0.
func TestMarkPublishedClearsDirtyAndRecordsRef(t *testing.T) {
	m, db := setupModule(t)

	site := &sitemanager.Site{Slug: "to-publish", Name: "To Publish", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	if err := m.MarkDirty(site.Id); err != nil {
		t.Fatalf("MarkDirty failed: %v", err)
	}

	ref := "git-commit-abc1234"
	if err := m.MarkPublished(site.Id, ref); err != nil {
		t.Fatalf("MarkPublished failed: %v", err)
	}

	var reloaded sitemanager.Site
	if err := db.Query(&reloaded).Where(sitemanager.Site_.Id).Eq(site.Id).ReadOne(); err != nil {
		t.Fatalf("ReadOne failed: %v", err)
	}

	if reloaded.Dirty {
		t.Fatal("expected Dirty == false, got true")
	}
	if reloaded.PublishedRef != ref {
		t.Fatalf("expected PublishedRef %q, got %q", ref, reloaded.PublishedRef)
	}
	if reloaded.PublishedAt <= 0 {
		t.Fatalf("expected PublishedAt > 0, got %d", reloaded.PublishedAt)
	}
}

// 9. Un borrador publicado queda StatusLive.
func TestMarkPublishedPromotesDraftToLive(t *testing.T) {
	m, db := setupModule(t)

	site := &sitemanager.Site{Slug: "draft-site", Name: "Draft Site", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	if sitemanager.Status(site.Status) != sitemanager.StatusDraft {
		t.Fatalf("expected initial status StatusDraft, got %v", site.Status)
	}

	if err := m.MarkPublished(site.Id, "ref-v1"); err != nil {
		t.Fatalf("MarkPublished failed: %v", err)
	}

	var reloaded sitemanager.Site
	if err := db.Query(&reloaded).Where(sitemanager.Site_.Id).Eq(site.Id).ReadOne(); err != nil {
		t.Fatalf("ReadOne failed: %v", err)
	}

	if sitemanager.Status(reloaded.Status) != sitemanager.StatusLive {
		t.Fatalf("expected promoted status StatusLive, got %v", reloaded.Status)
	}
}

// 10. Un suspendido publicado sigue StatusSuspended.
func TestMarkPublishedDoesNotPromoteSuspended(t *testing.T) {
	m, db := setupModule(t)

	site := &sitemanager.Site{Slug: "suspended-site", Name: "Suspended Site", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	if err := m.UpdateSiteStatus(site.Id, sitemanager.StatusLive); err != nil {
		t.Fatalf("UpdateSiteStatus Live failed: %v", err)
	}
	if err := m.UpdateSiteStatus(site.Id, sitemanager.StatusSuspended); err != nil {
		t.Fatalf("UpdateSiteStatus Suspended failed: %v", err)
	}

	if err := m.MarkPublished(site.Id, "ref-v2"); err != nil {
		t.Fatalf("MarkPublished failed: %v", err)
	}

	var reloaded sitemanager.Site
	if err := db.Query(&reloaded).Where(sitemanager.Site_.Id).Eq(site.Id).ReadOne(); err != nil {
		t.Fatalf("ReadOne failed: %v", err)
	}

	if sitemanager.Status(reloaded.Status) != sitemanager.StatusSuspended {
		t.Fatalf("expected status to remain StatusSuspended, got %v", reloaded.Status)
	}
}

// 11. ref == "" -> ErrInvalidData y el sitio no cambia.
func TestMarkPublishedRequiresRef(t *testing.T) {
	m, db := setupModule(t)

	site := &sitemanager.Site{Slug: "require-ref", Name: "Require Ref", Theme: "landing"}
	if err := m.CreateSite(site); err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	if err := m.MarkDirty(site.Id); err != nil {
		t.Fatalf("MarkDirty failed: %v", err)
	}

	// siteID empty check
	if err := m.MarkPublished("", "ref-x"); err != sitemanager.ErrInvalidData {
		t.Fatalf("expected ErrInvalidData for empty siteID, got %v", err)
	}

	// ref empty check
	err := m.MarkPublished(site.Id, "")
	if err != sitemanager.ErrInvalidData {
		t.Fatalf("expected ErrInvalidData for empty ref, got %v", err)
	}

	// Verify the site in the database was not modified
	var reloaded sitemanager.Site
	if err := db.Query(&reloaded).Where(sitemanager.Site_.Id).Eq(site.Id).ReadOne(); err != nil {
		t.Fatalf("ReadOne failed: %v", err)
	}

	if !reloaded.Dirty {
		t.Fatal("expected site to remain Dirty == true after failed MarkPublished")
	}
	if reloaded.PublishedRef != "" {
		t.Fatalf("expected PublishedRef to remain empty, got %q", reloaded.PublishedRef)
	}
	if reloaded.PublishedAt != 0 {
		t.Fatalf("expected PublishedAt to remain 0, got %d", reloaded.PublishedAt)
	}
}
