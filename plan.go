package sitemanager

import (
	"github.com/tinywasm/orm"
)

// CreatePlan registra un plan con sus límites.
func (m *Module) CreatePlan(p *Plan) error {
	if p == nil || p.Name == "" || p.MaxPages < 0 || p.MaxImages < 0 {
		return ErrInvalidData
	}

	var existing Plan
	err := m.db.Query(&existing).Where(Plan_.Name).Eq(p.Name).ReadOne()
	if err == nil {
		return ErrAlreadyExists
	} else if err != orm.ErrNotFound {
		return err
	}

	return m.db.Create(p)
}

// PlanOf devuelve los límites del plan asignado al sitio.
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
		if err == orm.ErrNotFound {
			return Plan{}, ErrNotFound
		}
		return Plan{}, err
	}

	return plan, nil
}
