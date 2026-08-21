package tests

import (
	"testing"

	sitemanager "github.com/veltylabs/site_manager"
)

func TestSiteByIDReturnsSite(t *testing.T) {
	sm, _ := setupModule(t)

	site := &sitemanager.Site{
		Slug:  "mi-sitio",
		Name:  "Mi Sitio",
		Theme: "modern",
	}

	err := sm.CreateSite(site)
	if err != nil {
		t.Fatalf("CreateSite fallo: %v", err)
	}

	fetched, err := sm.SiteByID(site.Id)
	if err != nil {
		t.Fatalf("SiteByID fallo: %v", err)
	}

	if fetched.Slug != "mi-sitio" {
		t.Errorf("se esperaba Slug 'mi-sitio', se obtuvo %s", fetched.Slug)
	}
	if fetched.Theme != "modern" {
		t.Errorf("se esperaba Theme 'modern', se obtuvo %s", fetched.Theme)
	}
}

func TestSiteByIDNotFound(t *testing.T) {
	sm, _ := setupModule(t)

	_, err := sm.SiteByID("")
	if err != sitemanager.ErrInvalidData {
		t.Errorf("se esperaba ErrInvalidData para id vacio, se obtuvo %v", err)
	}

	_, err = sm.SiteByID("inexistente")
	if err != sitemanager.ErrNotFound {
		t.Errorf("se esperaba ErrNotFound para id inexistente, se obtuvo %v", err)
	}
}

func TestCreatePlanRejectsInvalidData(t *testing.T) {
	sm, _ := setupModule(t)

	err := sm.CreatePlan(nil)
	if err != sitemanager.ErrInvalidData {
		t.Errorf("se esperaba ErrInvalidData para nil, se obtuvo %v", err)
	}

	err = sm.CreatePlan(&sitemanager.Plan{Name: "", MaxPages: 10, MaxImages: 10})
	if err != sitemanager.ErrInvalidData {
		t.Errorf("se esperaba ErrInvalidData para nombre vacio, se obtuvo %v", err)
	}

	err = sm.CreatePlan(&sitemanager.Plan{Name: "pro", MaxPages: -1, MaxImages: 10})
	if err != sitemanager.ErrInvalidData {
		t.Errorf("se esperaba ErrInvalidData para MaxPages negativo, se obtuvo %v", err)
	}

	err = sm.CreatePlan(&sitemanager.Plan{Name: "pro", MaxPages: 10, MaxImages: -1})
	if err != sitemanager.ErrInvalidData {
		t.Errorf("se esperaba ErrInvalidData para MaxImages negativo, se obtuvo %v", err)
	}
}

func TestCreatePlanRejectsDuplicateName(t *testing.T) {
	sm, _ := setupModule(t)

	p := &sitemanager.Plan{
		Name:      "basic",
		MaxPages:  5,
		MaxImages: 20,
	}

	err := sm.CreatePlan(p)
	if err != nil {
		t.Fatalf("CreatePlan fallo: %v", err)
	}

	err = sm.CreatePlan(p)
	if err != sitemanager.ErrAlreadyExists {
		t.Errorf("se esperaba ErrAlreadyExists para plan duplicado, se obtuvo %v", err)
	}
}

func TestPlanOfReturnsLimits(t *testing.T) {
	sm, _ := setupModule(t)

	p := &sitemanager.Plan{
		Name:         "pro",
		MaxPages:     50,
		MaxImages:    500,
		CustomDomain: true,
	}
	if err := sm.CreatePlan(p); err != nil {
		t.Fatalf("CreatePlan fallo: %v", err)
	}

	site := &sitemanager.Site{
		Slug:  "sitio-pro",
		Name:  "Sitio Pro",
		Theme: "pro-theme",
		Plan:  "pro",
	}
	if err := sm.CreateSite(site); err != nil {
		t.Fatalf("CreateSite fallo: %v", err)
	}

	plan, err := sm.PlanOf(site.Id)
	if err != nil {
		t.Fatalf("PlanOf fallo: %v", err)
	}

	if plan.Name != "pro" {
		t.Errorf("se esperaba Name 'pro', se obtuvo %s", plan.Name)
	}
	if plan.MaxPages != 50 {
		t.Errorf("se esperaba MaxPages 50, se obtuvo %d", plan.MaxPages)
	}
	if plan.MaxImages != 500 {
		t.Errorf("se esperaba MaxImages 500, se obtuvo %d", plan.MaxImages)
	}
	if !plan.CustomDomain {
		t.Errorf("se esperaba CustomDomain true, se obtuvo false")
	}
}

func TestPlanOfSiteWithoutPlan(t *testing.T) {
	sm, _ := setupModule(t)

	site := &sitemanager.Site{
		Slug:  "sitio-sin-plan",
		Name:  "Sitio Sin Plan",
		Theme: "basic",
	}
	if err := sm.CreateSite(site); err != nil {
		t.Fatalf("CreateSite fallo: %v", err)
	}

	_, err := sm.PlanOf(site.Id)
	if err != sitemanager.ErrNotFound {
		t.Errorf("se esperaba ErrNotFound para sitio sin plan asignado, se obtuvo %v", err)
	}
}

func TestPlanOfMissingPlanRow(t *testing.T) {
	sm, _ := setupModule(t)

	site := &sitemanager.Site{
		Slug:  "sitio-plan-fantasma",
		Name:  "Sitio Plan Fantasma",
		Theme: "basic",
		Plan:  "invalido",
	}
	if err := sm.CreateSite(site); err != nil {
		t.Fatalf("CreateSite fallo: %v", err)
	}

	_, err := sm.PlanOf(site.Id)
	if err != sitemanager.ErrNotFound {
		t.Errorf("se esperaba ErrNotFound para plan inexistente en la tabla, se obtuvo %v", err)
	}
}

func TestPlanOfUnknownSite(t *testing.T) {
	sm, _ := setupModule(t)

	_, err := sm.PlanOf("id-inexistente")
	if err != sitemanager.ErrNotFound {
		t.Errorf("se esperaba ErrNotFound para id de sitio inexistente, se obtuvo %v", err)
	}
}
