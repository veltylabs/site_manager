---
PLAN: "feat: leer un sitio por id y los límites de su plan"
TAG: v0.3.0
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 3798858568747240777
PR: https://github.com/veltylabs/site_manager/pull/3
---

> Plan autocontenido: todo lo necesario para ejecutarlo está aquí.
> Se despacha con el flujo CodeJob. Ver skill: agents-workflow.
> Reglas del repo: [`AGENTS.md`](../AGENTS.md) en la raíz — léelo antes de tocar
> nada (lista blanca de imports, sin stdlib, tests en `tests/`).

# Plan — el módulo no deja leer un sitio ni su plan

**Requisito previo**, porque este entorno no lo trae instalado:

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

## 1. El problema, con el caso real que lo destapó

`Plan` existe como modelo —`Name`, `MaxPages`, `MaxImages`, `CustomDomain`—, la
tabla se crea en `New()`, y `Site.Plan` guarda el nombre del plan del sitio.
**Nada de eso es alcanzable desde fuera:**

```sh
grep -rn "func (m \*Module)" *.go
# CreateSite, UpdateSiteStatus, MarkDirty, DirtySites, MarkPublished,
# Request, AcceptRequest, MemberOf, SitesOf, AddMember — y ninguna más
```

No hay forma de crear un `Plan`, ni de saber cuál es el plan de un sitio, ni
siquiera de leer un `Site` por su id.

El panel `veltylabs/misitio` lo destapó al implementar el cupo de imágenes. Su
especificación dice que una subida que excede `Plan.MaxImages` responde `409`.
Como el módulo no lo expone, el agente escribió esto:

```go
maxImages := 20 // plan max images default
```

Un límite de negocio inventado en la raíz de composición, que además contradice
la regla 1 del ecosistema: el dominio no se parchea en la app.

Lo mismo va a pasar en cuanto se implante la publicación: el exportador necesita
leer el `Slug` y el `Theme` de un sitio a partir de su id, y hoy sólo puede
llegar a un `Site` por la lista de membresías de un usuario — y la publicación
la ejecuta CI, que no es ningún usuario.

## 2. Qué se añade

Se amplía **`site.go`** con dos métodos y **`plan.go`** —hoy sólo un comentario—
con uno. Nada más: ni modelos nuevos, ni campos, ni cambios en `models.go`.

```go
// SiteByID devuelve el sitio con ese id.
func (m *Module) SiteByID(siteID string) (*Site, error)      // site.go

// CreatePlan registra un plan con sus límites.
func (m *Module) CreatePlan(p *Plan) error                    // plan.go

// PlanOf devuelve los límites del plan asignado al sitio.
func (m *Module) PlanOf(siteID string) (Plan, error)          // plan.go
```

### 2.1 — `SiteByID(siteID string) (*Site, error)` — en `site.go`

1. `siteID == ""` → `ErrInvalidData`.
2. Leer por `Site_.Id`. Si `ReadOne` devuelve `orm.ErrNotFound` → `ErrNotFound`;
   cualquier otro error se devuelve tal cual.
3. Devolver el sitio.

Es el mismo patrón de lectura que ya usan `MarkDirty` y `MarkPublished` en
[`publish.go`](../publish.go): **cópialo de ahí**, incluida la traducción del
error, para que las tres lecturas se comporten igual.

### 2.2 — `CreatePlan(p *Plan) error` — en `plan.go`

1. `p == nil`, `p.Name == ""`, `p.MaxPages < 0` o `p.MaxImages < 0` →
   `ErrInvalidData`. Un límite negativo no es "sin límite", es un dato roto.
2. Si ya existe un plan con ese `Name` → `ErrAlreadyExists`. `Name` es la clave
   primaria de la tabla (`PlanModel`), igual que `Slug` lo es de hecho para
   `Site`: mismo criterio que `CreateSite`.
3. `m.db.Create(p)`.

**El plan no lleva id generado**: su clave es el nombre. No llames a
`m.ids.NewID()` aquí.

### 2.3 — `PlanOf(siteID string) (Plan, error)` — en `plan.go`

1. `SiteByID(siteID)` — reutiliza el método de §2.1, no repitas la consulta.
   Propaga su error tal cual (`ErrInvalidData` / `ErrNotFound`).
2. Si `site.Plan == ""` → `ErrNotFound`. Un sitio sin plan asignado **no tiene
   límites que consultar**, y quien pregunta debe decidir qué hacer; devolver un
   `Plan` en blanco haría pasar `MaxImages = 0` por "sin cupo" o por "sin
   límite" según quién lo lea. Que falle es lo que evita esa ambigüedad.
3. Leer el plan por `Plan_.Name` igual a `site.Plan`. Si no existe → `ErrNotFound`.
4. Devolver el `Plan` **por valor**: son cuatro campos inmutables de
   configuración, y devolver un puntero invitaría a mutarlos.

## 3. Tests — `tests/plan_test.go`

`gotest`, nunca `go test`. En `tests/`, con el `setupModule(t)` que ya existe en
`tests/conformance_test.go`, y sólo aserciones de la stdlib de testing.

| Test | Qué fija |
|---|---|
| `TestSiteByIDReturnsSite` | Devuelve el sitio creado, con su `Slug` y su `Theme`. |
| `TestSiteByIDNotFound` | `ErrInvalidData` con `""`; `ErrNotFound` con un id inexistente. |
| `TestCreatePlanRejectsInvalidData` | `nil`, nombre vacío y un límite negativo → `ErrInvalidData`. |
| `TestCreatePlanRejectsDuplicateName` | Segundo plan con el mismo `Name` → `ErrAlreadyExists`. |
| `TestPlanOfReturnsLimits` | Sitio con plan asignado → los `MaxImages`/`MaxPages` de ese plan. |
| `TestPlanOfSiteWithoutPlan` | Sitio con `Plan == ""` → `ErrNotFound`. |
| `TestPlanOfMissingPlanRow` | Sitio que apunta a un plan que no existe → `ErrNotFound`. |
| `TestPlanOfUnknownSite` | Id inexistente → `ErrNotFound`. |

## 4. Documentación

- [`README.md`](../README.md): añade las tres firmas a la sección **API
  Pública**, y `plan.go` a la lista de archivos clave con su nueva
  responsabilidad.
- [`docs/ARCHITECTURE.md`](ARCHITECTURE.md): en las entidades del dominio, deja
  escrito que `Site.Plan` es el **nombre** del plan —la clave de `Plan`—, y que
  los límites se consultan con `PlanOf`.

Ningún documento debe citar `docs/PLAN.md`: este archivo se borra al publicar.

## 5. Criterios de aceptación

- [ ] `gotest` en verde.
- [ ] `gofmt -l .` vacío.
- [ ] `grep -rn "SiteByID\|CreatePlan\|PlanOf" *.go` → sólo `site.go` y `plan.go`.
- [ ] `grep -rn "map\[" *.go` → vacío. Este módulo compila con TinyGo dentro de un Worker.
- [ ] `grep -rn "\"time\"\|\"strings\"\|\"errors\"\|\"fmt\"\|encoding/json" *.go` → vacío: se usan `github.com/tinywasm/fmt` y `github.com/tinywasm/time`.
- [ ] `models.go`, `models_orm.go` y `publish.go` **sin cambios**: `git diff` no los toca.
- [ ] Los ocho tests de §3 existen y pasan.

## 6. Anti-footguns

1. **No agregues modelos ni campos.** `Plan` y `Site` ya están completos; esto es
   sólo API de lectura y un constructor.
2. **No inventes un plan por defecto.** Si un sitio no tiene plan, el error es la
   respuesta correcta: quien pregunta decide. Un `Plan` vacío de cortesía
   convierte "sin plan" en "cupo cero" o en "sin límite" según quién lo lea, y
   ese es justo el fallo silencioso que este plan viene a cerrar.
3. **Sin stdlib.** Este módulo se compila con TinyGo dentro de un Cloudflare
   Worker con un límite duro de 1 MB, y el panel que lo importa ya va por el
   69 % de ese presupuesto. Sin `map`, sin `reflect`, sin `encoding/json`.
4. **Sin backend concreto fuera de los tests.** La lista blanca de `AGENTS.md`
   manda: el módulo recibe un `*orm.DB` y no sabe qué hay detrás.
5. `docs/PLAN.md` (este archivo) no se renombra ni se borra, y su bloque de
   frontmatter —`PLAN`, `TAG`, `EXECUTOR`, `STATUS`, `SESSION`, `PR`— **no se
   edita a mano**: lo escribe `codejob`.
