---
PLAN: "feat: registro de sitios, membresias y solicitudes de acceso"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 12430331083740875379
---

> Este plan se despacha con el flujo CodeJob. Ver skill: agents-workflow.

# Plan — `veltylabs/site_manager` v0.1.0

Crear el **registro de inquilinos** de `misitio`: el panel con el que los
clientes de Velty administran sus sitios web en `misitio.velty.cl`.

Este módulo es el dueño del `site_id`. Todo el aislamiento entre clientes del
producto se apoya en lo que se construya aquí.

---

## 0. Reglas de desarrollo — léelas completas antes de escribir código

Están en [`AGENTS.md`](../AGENTS.md), que es la plantilla canónica de todo
`veltylabs/modules/*`. Lo que sigue repite lo que **no** puedes dejar de aplicar,
porque un agente sin contexto no va a ir a buscarlo.

### 0.1 Lista blanca de importaciones

Los archivos **no-test** pueden importar, de `github.com/tinywasm/*`, sólo:

`model` · `router` · `view` · `events` · `orm` · `storage` · `ddl` ·
`form/input` · `fmt` · `time`

### 0.2 Lista negra — **ni siquiera en `_test.go`**

- Backends concretos: `tinywasm/sqlite`, `tinywasm/postgres`, `tinywasm/sqlt`,
  `tinywasm/indexdb`, cualquier driver `database/sql`.
  **Los tests usan `orm.New(mem.New())`** — `github.com/tinywasm/storage/mem`.
- Transportes concretos: `tinywasm/mcp`, `tinywasm/server`, `httpd`, `net/http`.
- Generador de IDs concreto: `tinywasm/unixid`. Recibe `model.IDGenerator` por
  inyección.
- Encoders concretos: `tinywasm/json`, `tinywasm/jsvalue`. Los modelos
  implementan `model.Encodable`/`Decodable` (los genera `ormc`); qué codec los
  recorre lo decide la app.
- Renderizadores: `tinywasm/layout` o cualquier kit de UI.
- Carpetas `internal/`.

### 0.3 Este módulo compila con **TinyGo dentro de un Worker**

Es la restricción que más fácil se olvida y la que revienta el despliegue:

- **Sin `map[K]V`.** TinyGo los compila mal e inflan el binario. Slices +
  búsqueda lineal (los conjuntos son chicos) o structs de campos fijos.
- **Sin `fmt`, `errors`, `strconv`, `strings`, `log`** de la stdlib. Usa
  `github.com/tinywasm/fmt`: `fmt.Errf`, `fmt.HasPrefix`,
  `fmt.Convert(s).TrimPrefix(p).String()`.
- **Sin `encoding/json`, sin `reflect`.**
- **`error` sí, `errors` no**: devolver `error` está bien; construirlo con
  `errors.New`/`fmt.Errorf` no.

El binario que hospeda este módulo tiene un límite **duro** de 1 MB impuesto por
Cloudflare. No es un presupuesto que se pueda subir: el despliegue falla.

### 0.4 Sin strings mágicos

Todo string repetido es una constante nombrada y exportada. Literales en la
lógica: prohibidos.

### 0.5 Idioma y estructura

Código en inglés; documentación y comentarios de prosa en español. Jerarquía
plana, archivos de menos de 500 líneas, todos los tests bajo `tests/`.

---

## 1. Qué construir

Cuatro entidades y las operaciones mínimas sobre ellas.

### 1.1 `Site` — `site.go`

| Campo | Tipo | Regla |
|---|---|---|
| `ID` | string | clave |
| `Slug` | string | **único**, `NotNull`. Es el nombre del directorio en el repo de sitios: sólo `[a-z0-9-]` |
| `Name` | string | `NotNull` |
| `Domain` | string | dominio propio, puede ir vacío |
| `DomainExpiresAt` | int64 | cero = sin dominio propio |
| `Plan` | string | nombre del plan |
| `Theme` | string | `NotNull`; hoy el único valor válido es `"landing"` |
| `Status` | `Status` | ver abajo |
| `Dirty` | bool | cola de publicación |
| `PublishedAt` | int64 | |
| `PublishedRef` | string | referencia del último publicado |

```go
type Status uint8

const (
	StatusDraft     Status = iota // ZERO VALUE — no publicado
	StatusLive
	StatusSuspended
)
```

**`StatusDraft` DEBE ser el zero value.** Un sitio recién creado no está
publicado *por construcción*, no porque alguien recordó poner un booleano. Si
esto se invierte, el modo de fallo es publicar sitios que nadie autorizó.

Transiciones válidas, como **tabla, no como cadena de `if`**:

| Desde | Hacia |
|---|---|
| `draft` | `live` |
| `live` | `suspended` |
| `suspended` | `live` |

Cualquier otra devuelve error. Mensaje textual:

```
site_manager: transicion invalida de %s a %s
```

### 1.2 `SiteMember` — `member.go`

| Campo | Tipo | Regla |
|---|---|---|
| `ID` | string | clave |
| `SiteID` | string | `NotNull` |
| `UserID` | string | `NotNull` |
| `Role` | `Role` | `RoleViewer` es el zero value |

```go
type Role uint8

const (
	RoleViewer Role = iota // ZERO VALUE — el permiso más bajo
	RoleEditor
	RoleOwner
)
```

`RoleViewer` como zero value por la misma razón: un rol mal leído o un campo
sin escribir tiene que degradar a "puede mirar", nunca a "puede publicar".

**La operación más importante del módulo:**

```go
// MemberOf responde si un usuario puede tocar un sitio, y con qué rol.
// Es la UNICA forma de responder esa pregunta: un consumidor que arme su
// propia consulta terminara con una regla distinta a la de los demas.
func (m *Module) MemberOf(userID, siteID string) (Role, bool)
```

Y su compañera, la que evita N+1 en el panel:

```go
// SitesOf devuelve los sitios donde el usuario tiene membresia.
// Puede devolver vacio: es un estado normal, no un error.
func (m *Module) SitesOf(userID string) ([]Site, error)
```

### 1.3 `Plan` — `plan.go`

| Campo | Tipo |
|---|---|
| `Name` | string |
| `MaxPages` | int |
| `MaxImages` | int |
| `CustomDomain` | bool |

**Sólo límites. Ni precios, ni monedas, ni ciclos de facturación, ni estados de
morosidad.** Cuando exista cobro será un módulo aparte que escribe `Site.Plan`.
No lo anticipes: diseñar facturación antes del primer cliente pagando es diseñar
contra una suposición.

### 1.4 `AccessRequest` — `access.go`

| Campo | Tipo | Regla |
|---|---|---|
| `ID` | string | clave |
| `Email` | string | `NotNull` — verificado por Google, no escrito a mano |
| `Name` | string | |
| `Message` | string | qué pide |
| `CreatedAt` | int64 | |
| `Status` | `RequestStatus` | `RequestPending` es el zero value |

```go
type RequestStatus uint8

const (
	RequestPending  RequestStatus = iota // ZERO VALUE
	RequestAccepted
	RequestRejected
)
```

**`AccessRequest` NO es autorización: es un embudo de venta.** Registrar una
solicitud **nunca** crea una `SiteMember`. Dar acceso es una decisión humana de
Velty; iniciar sesión con Google sólo prueba identidad.

```go
// Request registra una solicitud de acceso. Si ya existe una pendiente para
// ese correo, devuelve la existente y created=false: reintentar no puede
// generar un segundo aviso al equipo de Velty.
func (m *Module) Request(email, name, message string) (req AccessRequest, created bool, err error)
```

**Anti-footgun:** ninguna función de este módulo debe crear una `SiteMember`
como efecto secundario de aceptar una solicitud. Aceptar cambia
`RequestStatus`; crear el sitio y la membresía son llamadas separadas y
explícitas.

---

## 2. Estructura de archivos

```
site.go            // Site + Status + transiciones
member.go          // SiteMember + Role + MemberOf/SitesOf
plan.go            // Plan
access.go          // AccessRequest + Request
module.go          // Module, Deps, New(), Ops, CreateTable
errors.go          // errores centinela
*_orm.go           // GENERADOS por ormc — no editar a mano
docs/ARCHITECTURE.md
docs/diagrams/database.md
tests/
```

Borra `site_manager.go` (el archivo vacío del andamiaje). Verificación:
`ls site_manager.go` → no existe.

### `Module` y sus dependencias

```go
type Deps struct {
	DB  *orm.DB
	IDs model.IDGenerator // INYECTADO — nunca construido aqui
}

func New(d Deps) (*Module, error)
```

`New` devuelve error si falta cualquier dependencia. Un módulo a medias que
arranca es peor que uno que no arranca.

Chequeo de contrato en tiempo de compilación, junto a la implementación:

```go
var _ router.OpModule = (*Module)(nil)
```

---

## 3. Tests — `tests/`

Con `orm.New(mem.New())`. Ningún driver concreto, ni aquí.

| # | Caso | Espera |
|---|---|---|
| 1 | `Site` recién creado | `Status == StatusDraft` sin escribir el campo |
| 2 | `SiteMember` recién creado | `Role == RoleViewer` sin escribir el campo |
| 3 | `AccessRequest` recién creada | `Status == RequestPending` sin escribir el campo |
| 4 | transición `draft → suspended` | error con el mensaje textual de §1.1 |
| 5 | transición `draft → live` | ok |
| 6 | `MemberOf` de un usuario sin membresía | `(RoleViewer, false)` |
| 7 | `MemberOf` de un usuario con membresía | el rol correcto, `true` |
| 8 | `SitesOf` de un usuario sin sitios | slice vacío, **sin error** |
| 9 | `SitesOf` no devuelve sitios ajenos | ni uno |
| 10 | dos `Site` con el mismo `Slug` | error de unicidad |
| 11 | segunda `Request` con una pendiente del mismo correo | la existente, `created == false` |
| 12 | `Request` tras aceptar la anterior | una nueva, `created == true` |
| 13 | aceptar una `AccessRequest` | **cero** `SiteMember` creadas |

El caso 13 es el que protege la decisión de producto. Escríbelo aunque parezca
que prueba una ausencia: es exactamente lo que hay que proteger.

---

## 4. Documentación

- `docs/ARCHITECTURE.md` — alcance del dominio, las cuatro entidades, tabla de
  Ops, ejemplo de raíz de composición. **Sin código de implementación.**
- `docs/diagrams/database.md` — ERD en Mermaid. **Nunca uses `subgraph`**:
  rompe el renderizado en el TUI. Usa `flowchart TD` y `<br/>` para los saltos.
- `README.md` — inicio rápido, tabla de Ops, archivos clave. Debe enlazar todo
  lo que haya en `docs/` **excepto** `PLAN.md`, que es efímero.

---

## 5. Criterios de aceptación

- [ ] `go vet ./...` limpio; `go test ./tests/...` en verde con los 13 casos.
- [ ] `grep -rn "tinywasm/sqlite\|tinywasm/postgres\|tinywasm/sqlt\|net/http\|tinywasm/mcp\|tinywasm/unixid\|tinywasm/json\|tinywasm/jsvalue\|tinywasm/layout" .` → **vacío, tests incluidos**.
- [ ] `grep -rn "map\[" --include=*.go . | grep -v _test.go` → vacío.
- [ ] `grep -rn "encoding/json\|\"reflect\"\|\"strings\"\|\"errors\"\|\"strconv\"\|\"log\"" --include=*.go .` → vacío.
- [ ] `grep -rn "internal/" .` → vacío.
- [ ] `ls site_manager.go` → no existe.
- [ ] Códecs generados con `ormc`, commiteados.
- [ ] `docs/ARCHITECTURE.md`, `docs/diagrams/database.md` y `README.md` escritos.
- [ ] `grep -rn "subgraph" docs/` → vacío.

## 6. Fuera de alcance

Contenido del sitio (vive en `veltylabs/site_content`), plantillas
(`veltylabs/sitetheme`), facturación, y cualquier ruta HTTP: este módulo expone
`router.OpModule`, y quién lo monta lo decide la app.
