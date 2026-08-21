package sitemanager

import (
	"github.com/tinywasm/orm"
	"github.com/tinywasm/time"
)

// MarkDirty marca el sitio como pendiente de publicar. Es idempotente: si ya
// estaba sucio no vuelve a escribir.
func (m *Module) MarkDirty(siteID string) error {
	if siteID == "" {
		return ErrInvalidData
	}

	var site Site
	err := m.db.Query(&site).Where(Site_.Id).Eq(siteID).ReadOne()
	if err != nil {
		if err == orm.ErrNotFound {
			return ErrNotFound
		}
		return err
	}

	if site.Dirty {
		return nil
	}

	site.Dirty = true
	return m.db.Update(&site, orm.Eq(Site_.Id, siteID))
}

// DirtySites devuelve los sitios pendientes de publicar, excluyendo los
// suspendidos.
func (m *Module) DirtySites() (SiteList, error) {
	return ReadAllSite(
		m.db.Query(&Site{}).
			Where(Site_.Dirty).Eq(true).
			Where(Site_.Status).Neq(int64(StatusSuspended)),
	)
}

// MarkPublished registra una publicación exitosa: limpia Dirty, guarda cuándo y
// con qué referencia, y promueve el sitio a Live si era un borrador.
//
// Ventana conocida: si el cliente guarda contenido mientras CI construye,
// MarkPublished limpiará un Dirty que corresponde a un cambio más nuevo, y ese
// cambio esperará al cron diario de publicación. Resolverlo de verdad exige
// versionar el contenido, que hoy no existe.
func (m *Module) MarkPublished(siteID, ref string) error {
	if siteID == "" || ref == "" {
		return ErrInvalidData
	}

	var site Site
	err := m.db.Query(&site).Where(Site_.Id).Eq(siteID).ReadOne()
	if err != nil {
		if err == orm.ErrNotFound {
			return ErrNotFound
		}
		return err
	}

	site.Dirty = false
	site.PublishedAt = time.Now() / 1_000_000_000
	site.PublishedRef = ref

	if Status(site.Status) == StatusDraft {
		site.Status = int64(StatusLive)
	}

	return m.db.Update(&site, orm.Eq(Site_.Id, siteID))
}
