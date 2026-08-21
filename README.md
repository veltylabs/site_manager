# site_manager
<img src="docs/img/badges.svg">

Registro de sitios, membresías, planes y solicitudes de acceso para clientes de Velty (`misitio.velty.cl`).

Este módulo es el dueño del `site_id` dentro del ecosistema de Velty.

## Documentación

- [Arquitectura](docs/ARCHITECTURE.md)
- [Diagrama ERD de Base de Datos](docs/diagrams/database.md)

## API Pública (Public API)

### Publicación de Sitios

- `MarkDirty(siteID string) error`: Marca el sitio como pendiente de publicar (`Dirty = true`). Es idempotente.
- `DirtySites() (SiteList, error)`: Devuelve la lista de sitios pendientes de publicar, excluyendo los suspendidos.
- `MarkPublished(siteID, ref string) error`: Registra una publicación exitosa, limpia `Dirty`, guarda la marca de tiempo `PublishedAt` y la referencia `PublishedRef`, y promueve el estado de `draft` a `live`.

## Operaciones (Ops)

| Op Name | Recurso | Acción | Descripción |
|---|---|---|---|
| `site_get` | `site` | `read` | Consulta de sitio por ID |
| `site_create` | `site` | `create` | Creación de sitio con verificación de slug |
| `access_request` | Public | - | Registro de solicitud de acceso |

## Archivos clave

- `site.go` — Definición de `Site`, estados `Status` y transiciones válidas.
- `publish.go` — Estado de publicación (`MarkDirty`, `DirtySites`, `MarkPublished`).
- `member.go` — Definición de `SiteMember`, roles `Role`, y métodos `MemberOf` / `SitesOf`.
- `plan.go` — Definición de límites del `Plan`.
- `access.go` — Definición de `AccessRequest` y solicitudes de acceso.
- `module.go` — Estructura `Module`, constructor `New`, montado de `MountOps` y operaciones.
- `errors.go` — Errores centinela del módulo.
- `models.go` — Literales `model.Definition` para generación de código con `ormc`.
- `models_orm.go` — Código ORM generado automáticamente.
- `tests/conformance_test.go` — Suite completa de pruebas de conformidad e integración.
- `tests/publish_test.go` — Pruebas del ciclo de publicación.

## Inicio Rápido

```go
package main

import (
	"github.com/tinywasm/orm"
	"github.com/tinywasm/storage/mem"
	sitemanager "github.com/veltylabs/site_manager"
)

func main() {
	db := orm.New(mem.New())
	sm, err := sitemanager.New(sitemanager.Deps{
		DB:  db,
		IDs: myIDGenerator,
	})
	if err != nil {
		panic(err)
	}

	role, ok := sm.MemberOf("user_123", "site_456")
	_ = role
	_ = ok
}
```
