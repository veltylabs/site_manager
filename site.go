package sitemanager

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
)

type Status uint8

const (
	StatusDraft Status = iota // ZERO VALUE — no publicado
	StatusLive
	StatusSuspended
)

func (s Status) String() string {
	switch s {
	case StatusDraft:
		return "draft"
	case StatusLive:
		return "live"
	case StatusSuspended:
		return "suspended"
	default:
		return "unknown"
	}
}

type transition struct {
	from Status
	to   Status
}

var validTransitions = []transition{
	{from: StatusDraft, to: StatusLive},
	{from: StatusLive, to: StatusSuspended},
	{from: StatusSuspended, to: StatusLive},
}

func ValidateTransition(from, to Status) error {
	for i := 0; i < len(validTransitions); i++ {
		t := validTransitions[i]
		if t.from == from && t.to == to {
			return nil
		}
	}
	return fmt.Errf("site_manager: transicion invalida de %s a %s", from.String(), to.String())
}

// CreateSite creates a new site ensuring slug uniqueness.
func (m *Module) CreateSite(site *Site) error {
	if site == nil {
		return ErrInvalidData
	}
	if site.Slug == "" || site.Name == "" || site.Theme == "" {
		return ErrInvalidData
	}

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

// SiteByID devuelve el sitio con ese id.
func (m *Module) SiteByID(siteID string) (*Site, error) {
	if siteID == "" {
		return nil, ErrInvalidData
	}

	var site Site
	err := m.db.Query(&site).Where(Site_.Id).Eq(siteID).ReadOne()
	if err != nil {
		if err == orm.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &site, nil
}
