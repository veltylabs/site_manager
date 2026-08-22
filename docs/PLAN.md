---
PLAN: "feat: PendingRequests y RequestByID sobre AccessRequest"
TAG: v0.4.0
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 17612746035528647813
PR: https://github.com/veltylabs/site_manager/pull/4
---

> Este plan se despacha con el flujo CodeJob. Ver skill: agents-workflow.

# Plan — `veltylabs/site_manager`: listar y leer `AccessRequest`

## Contexto

`site_manager` ya expone `Request(email, name, message)` (crear) y `AcceptRequest(id)`
(marcar aceptada) sobre `AccessRequest`, en [`access.go`](../access.go). No expone
ninguna forma de **listar** las solicitudes pendientes ni de **leer una por id**.

Un consumidor de esta librería (`veltylabs/misitio`, la app de administración de
Velty) necesita construir una vista de super-administrador que liste las
solicitudes de acceso pendientes y permita aceptarlas. Sin estas dos funciones,
ese consumidor no tiene más remedio que consultar las tablas de `AccessRequest`
directamente desde su propio código — lo cual es exactamente el acoplamiento que
esta librería existe para evitar: el consumidor no debe conocer el esquema de
almacenamiento de `AccessRequest`, sólo su API pública.

Esto es una **puerta**: hasta que estas dos funciones estén publicadas, el
consumidor no puede completar su propia etapa.

## Objetivo

Agregar dos funciones exportadas sobre `*Module`, ambas de sólo lectura, ambas
en [`access.go`](../access.go), junto a `Request` y `AcceptRequest`:

```go
// PendingRequests devuelve las solicitudes de acceso sin resolver, de la más
// antigua a la más reciente.
func (m *Module) PendingRequests() ([]AccessRequest, error)

// RequestByID devuelve la solicitud de acceso con ese id.
func (m *Module) RequestByID(id string) (AccessRequest, error)
```

## Paso 1 — `PendingRequests()`

En [`access.go`](../access.go), después de `AcceptRequest`:

```go
// PendingRequests devuelve las solicitudes de acceso sin resolver, de la más
// antigua a la más reciente.
func (m *Module) PendingRequests() ([]AccessRequest, error) {
	rows, err := ReadAllAccessRequest(
		m.db.Query(&AccessRequest{}).
			Where(AccessRequest_.Status).Eq(int64(RequestPending)),
	)
	if err != nil {
		return nil, err
	}
	out := make([]AccessRequest, len(rows))
	for i := range rows {
		out[i] = *rows[i]
	}
	return out, nil
}
```

No ordena explícitamente por `CreatedAt` en el query builder — si el ORM no
soporta `.OrderBy(...)` en este query builder (comprueba antes de escribir: grep
`OrderBy` en el paquete `orm` que consume este módulo), deja el orden natural de
inserción, que en `mem`/D1 coincide con el de creación, y dilo en un comentario de
una línea. No inventes un `sort.Slice` con `fmt` — si hace falta ordenar de verdad,
eso es un hueco de `orm`, no de este módulo.

## Paso 2 — `RequestByID(id)`

Mismo archivo, después de `PendingRequests`:

```go
// RequestByID devuelve la solicitud de acceso con ese id.
func (m *Module) RequestByID(id string) (AccessRequest, error) {
	var req AccessRequest
	err := m.db.Query(&req).Where(AccessRequest_.Id).Eq(id).ReadOne()
	if err != nil {
		return AccessRequest{}, err
	}
	return req, nil
}
```

Mismo patrón de lectura que ya usa `AcceptRequest` en este archivo — reutiliza
exactamente esa forma, no una distinta. Si el id no existe, `ReadOne` ya devuelve
un error (verificado por el test existente `TestAcceptRequestNotFound`, que
depende del mismo comportamiento) — no agregues una comprobación adicional ni un
tipo de error nuevo.

## Paso 3 — Tests

Archivo nuevo `tests/access_test.go` (no existe hoy; `Request`/`AcceptRequest`
sólo tienen guard-clauses cubiertas en `tests/module_test.go`). Sigue el patrón
exacto de `tests/conformance_test.go` y `tests/module_test.go`: paquete `tests`,
`setupModule(t)` para construir el módulo sobre `orm.New(mem.New())`.

```go
package tests

import (
	"testing"

	sitemanager "github.com/veltylabs/site_manager"
)

func TestPendingRequestsReturnsOnlyPending(t *testing.T) {
	m, _ := setupModule(t)

	first, _, err := m.Request("a@example.com", "A", "hola")
	if err != nil {
		t.Fatalf("Request(a): %v", err)
	}
	if _, _, err := m.Request("b@example.com", "B", "hola"); err != nil {
		t.Fatalf("Request(b): %v", err)
	}

	if err := m.AcceptRequest(first.Id); err != nil {
		t.Fatalf("AcceptRequest: %v", err)
	}

	pending, err := m.PendingRequests()
	if err != nil {
		t.Fatalf("PendingRequests: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(pending))
	}
	if pending[0].Email != "b@example.com" {
		t.Fatalf("expected pending request from b@example.com, got %s", pending[0].Email)
	}
}

func TestPendingRequestsEmpty(t *testing.T) {
	m, _ := setupModule(t)

	pending, err := m.PendingRequests()
	if err != nil {
		t.Fatalf("PendingRequests: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending requests, got %d", len(pending))
	}
}

func TestRequestByIDReturnsMatch(t *testing.T) {
	m, _ := setupModule(t)

	req, _, err := m.Request("a@example.com", "A", "hola")
	if err != nil {
		t.Fatalf("Request: %v", err)
	}

	got, err := m.RequestByID(req.Id)
	if err != nil {
		t.Fatalf("RequestByID: %v", err)
	}
	if got.Id != req.Id || got.Email != "a@example.com" {
		t.Fatalf("expected request %s (a@example.com), got %s (%s)", req.Id, got.Id, got.Email)
	}
}

func TestRequestByIDNotFound(t *testing.T) {
	m, _ := setupModule(t)

	if _, err := m.RequestByID("does-not-exist"); err == nil {
		t.Fatal("expected error for nonexistent id, got nil")
	}
}
```

## Reglas del ecosistema — ya las cumple el patrón existente, no las rompas

Este módulo es un `veltylabs/modules/*` (ver `AGENTS.md` en la raíz del repo).
Ambas funciones nuevas siguen exactamente el estilo de `AcceptRequest`: sólo
importan `github.com/tinywasm/orm` (ya importado en `access.go`), nada de
`storage/mem` fuera de `tests/`, nada de `fmt`/`errors`/`strconv` de la stdlib
(este paquete no los necesita para esto), ningún transporte ni renderer.

## Criterios de aceptación

- [ ] `PendingRequests()` y `RequestByID(id)` en `access.go`, mismo estilo que
      `Request`/`AcceptRequest`.
- [ ] `TestPendingRequestsReturnsOnlyPending`: dos solicitudes creadas, una
      aceptada, `PendingRequests()` devuelve **una**, y es la que sigue pendiente.
- [ ] `TestPendingRequestsEmpty`: sin solicitudes, devuelve slice vacío sin error.
- [ ] `TestRequestByIDReturnsMatch`: devuelve la solicitud correcta por id.
- [ ] `TestRequestByIDNotFound`: id inexistente devuelve error.
- [ ] `go test ./...` en verde, incluida la suite existente sin modificar.
- [ ] Se publica como **v0.4.0** (ver `TAG` en el frontmatter de este plan).

## Fuera de alcance

No se toca `Request`, `AcceptRequest`, ni ninguna otra función existente. No se
agrega paginación, filtrado por fecha, ni un tipo de error nuevo — si algo de
esto hace falta más adelante, es un plan aparte.
