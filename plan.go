package sitemanager

import (
	"webtyp.com/orm"
)

// CreatePlan registra un plan con sus límites.
//
// Ejemplo: err := m.CreatePlan(plan)
func (m *Module) CreatePlan(p *Plan) error {
	if p == nil || p.Name == "" || p.MaxPages < 0 || p.MaxImages < 0 {
		return ErrInvalidData
	}

	var existing Plan
	err := m.db.Query(&existing).Where(Plan_.Name).Eq(p.Name).ReadOne()
	if err == nil {
		return ErrAlreadyExists
	} else if !orm.IsNotFound(err) {
		return err
	}

	return m.db.Create(p)
}

// PlanOf devuelve los límites del plan asignado al sitio.
//
// Ejemplo: plan, err := m.PlanOf("site_123")
func (m *Module) PlanOf(siteID string) (Plan, error) {
	site, err := m.SiteByID(siteID)
	if err != nil {
		return Plan{}, err
	}

	if site.Plan == "" {
		return Plan{}, ErrNotFound
	}

	var plan Plan
	err = m.db.Query(&plan).Where(Plan_.Name).Eq(site.Plan).ReadOne()
	if err != nil {
		if orm.IsNotFound(err) {
			return Plan{}, ErrNotFound
		}
		return Plan{}, err
	}

	return plan, nil
}
