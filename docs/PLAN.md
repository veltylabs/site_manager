---
PLAN: "fix: los ops leian cualquier sitio sin comprobar pertenencia"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 8672192147222686395
PR: https://github.com/veltylabs/site_manager/pull/5
---

> Este plan se despacha con el flujo CodeJob. Ver skill: agents-workflow.

# Plan — los ops no filtran por pertenencia (y uno es anónimo)

## El hallazgo

`MountOps` en [`module.go`](../module.go) registra tres operaciones. Las tres tienen un
defecto de acceso:

```go
reg.Op("site_get", func(ctx router.Context) {
	var s Site
	if err := ctx.Decode(&s); err != nil { ctx.WriteStatus(400); return }
	if err := m.db.Query(&s).Where(Site_.Id).Eq(s.Id).ReadOne(); err != nil {
		ctx.WriteStatus(404); return
	}
	_ = ctx.Encode(&s)
}).Requires(model.Resource("site"), model.Read)
```

**El id del sitio viene del cuerpo de la petición y no se comprueba contra nada.** Cualquier
llamante con `site:r` lee **cualquier** sitio. Lo mismo `site_create`, que crea sin dejar
ningún `SiteMember`, así que el sitio nace sin dueño.

Y `access_request` está declarado **`.Public()`**, tomando el correo y el nombre del cuerpo: un
formulario de solicitud anónimo, que es exactamente el vector de spam que el consumidor de este
módulo evita a mano tomando el correo del token verificado.

Hoy no es explotable: el único consumidor es `veltylabs/misitio`, que monta REST y hace el
control en sus propios handlers. **Deja de no serlo en cuanto alguien coseche estos ops con
`mcp.HarvestOps`.** Se cierra antes de que eso pase.

Es literalmente lo que prohíbe la sección *Multi-tenancy* del
[`AGENTS.md` canónico](https://github.com/veltylabs/modules/blob/main/AGENTS.md):

> toda condición UPDATE/DELETE incluye la columna de tenant — `orm.Eq(X_.Id, id)` a secas es
> una escritura cross-tenant esperando a ocurrir, y el patrón leer-luego-escribir es una
> ventana TOCTOU, no una defensa.

## Anti-footguns

- Este repo sigue el `AGENTS.md` canónico de `veltylabs/modules`: **whitelist de imports**
  (`model`, `router`, `view`, `events`, `orm`, `storage`, `ddl`, `form/input`, `fmt`, `time`),
  sin `map[K]V` ni en tests, sin `reflect`, sin stdlib que el ecosistema ya reemplaza.
- **No importes `veltylabs/iam` ni ningún cliente de identidad.** La identidad llega por
  `ctx.UserID()`, que el transporte ya resolvió. Un módulo no sabe quién autentica.
- **No importes otro módulo.** Si hace falta un dato de un vecino, se declara una interfaz
  lectora estrecha propia (patrón `CatalogReader`) — aquí no hace falta.
- **No cambies la firma de los métodos de servicio** (`MemberOf`, `SitesOf`, `CreateSite`,
  `Request`, …): `veltylabs/misitio` los llama directo desde sus handlers REST y este plan no
  debe romperlo. Lo que cambia es **el gate de los ops**, no el dominio.
- Repo de `veltylabs`: identificadores en inglés, comentarios y docs en español.

---

## Etapa 1 — `site_get` filtra por pertenencia

Archivo: **[`module.go`](../module.go)**.

```go
	reg.Op("site_get", func(ctx router.Context) {
		userID := ctx.UserID()
		if userID == "" {
			ctx.WriteStatus(401)
			return
		}

		var s Site
		if err := ctx.Decode(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		if s.Id == "" {
			ctx.WriteStatus(400)
			return
		}

		// La pertenencia es la mitad por fila del control de acceso: el gate
		// del enrutador concede site:r en general, esto decide CUAL sitio.
		// Sin esto, site:r lee cualquier sitio del sistema.
		if _, ok := m.MemberOf(userID, s.Id); !ok {
			ctx.WriteStatus(403)
			return
		}

		if err := m.db.Query(&s).Where(Site_.Id).Eq(s.Id).ReadOne(); err != nil {
			ctx.WriteStatus(404)
			return
		}
		_ = ctx.Encode(&s)
	}).Requires(model.Resource("site"), model.Read).Accepts(&Site{})
```

`403` y no `404`: los identificadores no son adivinables, y un `404` mentiroso hace
indistinguible un fallo de permisos de un dato borrado.

## Etapa 2 — `site_create` deja dueño, o no deja nada

Mismo archivo. Un sitio sin `SiteMember` es un sitio que nadie puede administrar: nace
inaccesible y solo se arregla tocando la base a mano.

```go
	reg.Op("site_create", func(ctx router.Context) {
		userID := ctx.UserID()
		if userID == "" {
			ctx.WriteStatus(401)
			return
		}

		var s Site
		if err := ctx.Decode(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}
		if err := m.CreateSite(&s); err != nil {
			ctx.WriteStatus(400)
			return
		}

		// Sin esto el sitio nace sin dueño y solo se rescata tocando la base a
		// mano. No hay transacción: si falla, dilo en el mensaje — un 500 mudo
		// manda al operador a buscar a ciegas.
		if _, err := m.AddMember(s.Id, userID, RoleOwner); err != nil {
			ctx.WriteStatus(500)
			_ = ctx.Encode(&s)
			return
		}

		ctx.WriteStatus(201)
		_ = ctx.Encode(&s)
	}).Requires(model.Resource("site"), model.Create).Accepts(&Site{})
```

El `201` reemplaza al `200` implícito: crear devuelve `201`, y el
[`AGENTS.md` canónico](https://github.com/veltylabs/modules/blob/main/AGENTS.md) §Op handlers
fija esa convención.

**Ojo:** `veltylabs/misitio` tiene su propio flujo de alta que crea el sitio para **otro**
usuario (un administrador dando de alta a un cliente) y añade el `SiteMember` él mismo,
llamando a `CreateSite` y `AddMember` por separado. Ese camino **no pasa por este op** y no se
toca.

## Etapa 3 — `access_request` deja de ser anónimo

Mismo archivo. Hoy es `.Public()` y toma el correo del cuerpo: cualquiera puede llenar la tabla
de solicitudes desde fuera.

```go
	}).Authenticated().Accepts(&AccessRequest{})
```

`Authenticated` y no `Requires(...)`: solicitar acceso es una operación **sobre el propio
llamante**, y quien la hace por definición todavía no tiene ningún permiso sobre nada. Es el
caso que `model.AccessAuthenticated` describe.

**Residual, escríbelo en `docs/ARCHITECTURE.md`:** el correo sigue llegando en el cuerpo, así
que un llamante autenticado puede solicitar acceso a nombre de otra dirección. Cerrarlo exige
que el módulo pueda leer el correo verificado del llamante, y `router.Context` hoy solo expone
`UserID()`. **No inventes un puerto de identidad local para taparlo** — sería un contrato
duplicado, justo lo que el `AGENTS.md` canónico prohíbe. El primer consumidor que use este op
en serio abre el plan aguas arriba. (`veltylabs/misitio` no lo usa: su handler REST toma el
correo del token verificado.)

## Etapa 4 — Tests

Bajo **`tests/`** (paquete `tests`, externo), con `orm.New(mem.New())` y `router/mock` como
`OpRegistry` — nunca un backend concreto ni un transporte concreto.

| Test | Afirma |
|---|---|
| `TestSiteGetDeniesNonMember` | usuario con `site:r` y **sin** membresía → `403` |
| `TestSiteGetAllowsMember` | el miembro recibe `200` y el sitio |
| `TestSiteGetRejectsAnonymous` | sin identidad → `401` |
| `TestSiteGetRejectsEmptyID` | cuerpo sin `Id` → `400` |
| `TestSiteCreateAddsOwner` | tras `site_create`, `MemberOf(caller, nuevoID)` devuelve `RoleOwner` |
| `TestSiteCreateReturns201` | el estado es `201`, no `200` |
| `TestAccessRequestRejectsAnonymous` | sin identidad → `403` (lo escribe el enrutador) |
| `TestAccessRequestAllowsAuthenticated` | con identidad y sin roles → se crea |
| `TestOpsDeclareArgs` | las tres rutas de `Routes()` traen `Args` no nulo |

## Etapa 5 — Documentación

- **[`docs/ARCHITECTURE.md`](ARCHITECTURE.md)** — una sección corta: el control de acceso de
  este módulo tiene **dos mitades** (el gate del enrutador concede el recurso en general; la
  pertenencia decide la fila), y ninguna sustituye a la otra. Incluye el residual de la etapa 3.
- **`README.md`** — si documenta los ops, actualiza sus estados y su declaración de acceso.
- **No enlaces `docs/PLAN.md`** desde ningún documento permanente.

## Criterios de aceptación

- [ ] `go build ./...`, `go vet ./...` limpios; `gotest ./...` en verde.
- [ ] `GOOS=js GOARCH=wasm go vet ./...` limpio.
- [ ] `grep -n "reg.Op(" module.go` → tres, y **ninguna** seguida de `.Public()`.
- [ ] `grep -n "Accepts(" module.go` → tres.
- [ ] `grep -rn "map\[" .` → vacío (tests incluidos).
- [ ] `grep -rn "tinywasm/mcp\|tinywasm/json\|tinywasm/unixid\|tinywasm/sqlt\|tinywasm/postgres\|tinywasm/layout\|veltylabs/iam" .`
      → vacío.
- [ ] `MemberOf`, `SitesOf`, `CreateSite`, `AddMember`, `Request`, `AcceptRequest`,
      `PendingRequests` conservan su firma: `veltylabs/misitio` los llama directo.
- [ ] `docs/ARCHITECTURE.md` describe las dos mitades y el residual.

## Fuera de alcance

`view.Presenter` (no tiene consumidor todavía: el panel de misitio no puede moverlo sin un
`router.Caller` sobre HTTP, que no existe), cambios de esquema, y cualquier cambio a los
métodos de servicio.

## Etapas

| # | Etapa | Archivos |
|---|---|---|
| 1 | `site_get` filtra por pertenencia | `module.go` |
| 2 | `site_create` deja dueño | `module.go` |
| 3 | `access_request` autenticado | `module.go` |
| 4 | Tests | `tests/` |
| 5 | Documentación | `docs/ARCHITECTURE.md`, `README.md` |
