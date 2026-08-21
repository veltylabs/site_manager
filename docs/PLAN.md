---
PLAN: "feat: marcar y limpiar el estado de publicación de un sitio"
TAG: v0.2.0
EXECUTOR: jules
REVIEWER: none
---

> Plan autocontenido: todo lo necesario para ejecutarlo está aquí.
> Se despacha con el flujo CodeJob. Ver skill: agents-workflow.
> Reglas del repo: [`AGENTS.md`](../AGENTS.md) en la raíz — léelo antes de tocar
> nada (lista blanca de imports, sin stdlib, tests en `tests/`).

# Plan — el ciclo de publicación no tiene por dónde entrar

**Requisito previo**, porque este entorno no lo trae instalado:

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

## 1. El problema, con el caso real que lo destapó

`Site` tiene tres campos que describen su publicación:

```go
Dirty        bool   // hay cambios sin publicar
PublishedAt  int64  // cuándo se publicó por última vez
PublishedRef string // qué referencia (commit) quedó publicada
```

**Ningún método del módulo los escribe.** Compruébalo antes de empezar:

```sh
grep -rn "Dirty\|PublishedAt\|PublishedRef" *.go | grep -v models_orm.go
# hoy: vacío
```

Están en el modelo, en el esquema y en el codificador ORM, y son inalcanzables.

El panel `veltylabs/misitio` es quien lo destapó. Su flujo es: el cliente guarda
contenido → el sitio queda **sucio** → CI pregunta qué sitios están sucios, los
publica y avisa cuál fue la referencia publicada. Sin API para esos tres campos,
el agente que implementó el panel hizo lo único que encontró:

```go
_ = sm.UpdateSiteStatus(siteID, sitemanager.StatusDraft)
```

Que no funciona, por dos razones que conviene entender antes de escribir nada:

1. `Dirty` y `Status` son campos distintos. Esa llamada no toca `Dirty`.
2. `validTransitions` sólo admite `Draft→Live`, `Live→Suspended` y
   `Suspended→Live`. **No existe ninguna transición hacia `Draft`**, así que la
   llamada devuelve error siempre. Con el error descartado, guardar contenido
   respondía `200` sin haber marcado nada.

`Status` describe el ciclo de vida comercial del sitio — borrador, en línea,
suspendido. `Dirty` describe si lo que está en línea coincide con lo que hay en
la base. **Son ejes independientes** y este plan no los mezcla: un sitio `Live`
puede estar sucio, y publicarlo no lo saca de línea.

## 2. Qué se añade

Un archivo nuevo, `publish.go`, con **tres métodos** sobre `*Module`. Nada más:
ni campos nuevos en `Site`, ni cambios en `models.go`, ni en la tabla de
transiciones.

```go
// MarkDirty marca el sitio como pendiente de publicar. Es idempotente: si ya
// estaba sucio no vuelve a escribir.
func (m *Module) MarkDirty(siteID string) error

// DirtySites devuelve los sitios pendientes de publicar, excluyendo los
// suspendidos.
func (m *Module) DirtySites() (SiteList, error)

// MarkPublished registra una publicación exitosa: limpia Dirty, guarda cuándo y
// con qué referencia, y promueve el sitio a Live si era un borrador.
func (m *Module) MarkPublished(siteID, ref string) error
```

### 2.1 — `MarkDirty(siteID string) error`

1. `siteID == ""` → `ErrInvalidData`.
2. Leer el sitio por `Site_.Id`. Si no existe → `ErrNotFound`.
3. **Si `site.Dirty` ya es `true`, devolver `nil` sin escribir.** Cada guardado
   del panel llama aquí; una escritura por pulsación de "guardar" en un sitio ya
   sucio es una escritura de D1 que no cambia nada.
4. `site.Dirty = true` y `m.db.Update(&site, orm.Eq(Site_.Id, siteID))`.

**No toca `Status`.** Un sitio `Live` con cambios sin publicar sigue `Live`: lo
que está publicado sigue en línea hasta que la publicación lo reemplace.

### 2.2 — `DirtySites() (SiteList, error)`

Devuelve `SiteList` —el tipo que ya devuelve `ReadAllSite`, es decir `[]*Site`—,
no un *slice* de valores: así no se copia el struct ni se inventa un tipo nuevo.

La consulta, con los nombres exactos del constructor de queries:

```go
ReadAllSite(
    m.db.Query(&Site{}).
        Where(Site_.Dirty).Eq(true).
        Where(Site_.Status).Neq(int64(StatusSuspended)),
)
```

**El operador se llama `Neq`, no `Ne`.**

Por qué se excluyen: un sitio suspendido es una decisión administrativa —
impago, abuso—. CI publica exactamente lo que esta lista devuelve, así que
incluir un suspendido lo devolvería a producción por la puerta de atrás. Que
siga marcado sucio es correcto: cuando se reactive, se publicará.

Es el mismo patrón que `Request` usa con `ReadAllAccessRequest`. Sin resultados →
*slice* vacío y `nil`, **nunca** `ErrNotFound`: "no hay nada que publicar" es un
éxito, no un fallo.

### 2.3 — `MarkPublished(siteID, ref string) error`

1. `siteID == ""` o `ref == ""` → `ErrInvalidData`. La referencia es obligatoria:
   una publicación que no dice qué publicó no es auditable.
2. Leer el sitio. Si no existe → `ErrNotFound`.
3. `site.Dirty = false`.
4. `site.PublishedAt = time.Now() / 1_000_000_000` — segundos, el mismo idioma
   que `access.go` usa para `created_at`. `github.com/tinywasm/time`, no la
   stdlib.
5. `site.PublishedRef = ref`.
6. **Si `Status(site.Status) == StatusDraft`, promoverlo a `StatusLive`** — es la
   transición `Draft→Live` que la tabla ya admite, y es literalmente lo que
   significa haber publicado por primera vez. Si el estado es `Live`, se queda
   `Live`. Si es `Suspended`, **no se toca**: no se promueve un sitio suspendido.
7. Un solo `m.db.Update(...)` con todo lo anterior.

**Ventana conocida, y es deliberada:** si el cliente guarda contenido *mientras*
CI construye, `MarkPublished` limpiará un `Dirty` que corresponde a un cambio más
nuevo, y ese cambio esperará al cron diario de publicación. Resolverlo de verdad
exige versionar el contenido, que hoy no existe. **No inventes un mecanismo de
versiones en este plan**; deja el comentario en el método diciendo esto mismo.

## 3. Tests — `tests/publish_test.go`

`gotest`, nunca `go test`. Todos en `tests/`, sólo aserciones de la stdlib de
testing, con el mismo montaje que ya usa `tests/module_test.go`
(`orm.New(mem.New())` y el `testIDGenerator` que ya está ahí).

| Test | Qué fija |
|---|---|
| `TestMarkDirtySetsFlag` | Un sitio limpio queda `Dirty == true` en la base. |
| `TestMarkDirtyIsIdempotent` | Llamarlo dos veces no falla y el sitio sigue sucio. |
| `TestMarkDirtyDoesNotChangeStatus` | Un sitio `Live` marcado sucio **sigue** `Live`. Es la confusión que originó este plan. |
| `TestMarkDirtyNotFound` | `ErrNotFound` con un id inexistente; `ErrInvalidData` con `""`. |
| `TestDirtySitesReturnsOnlyDirty` | Con un sitio sucio y otro limpio, devuelve exactamente uno. |
| `TestDirtySitesExcludesSuspended` | Un sitio sucio y suspendido **no** aparece. |
| `TestDirtySitesEmptyIsNotAnError` | Sin sitios sucios: *slice* vacío y `err == nil`. |
| `TestMarkPublishedClearsDirtyAndRecordsRef` | `Dirty == false`, `PublishedRef` guardado, `PublishedAt > 0`. |
| `TestMarkPublishedPromotesDraftToLive` | Un borrador publicado queda `StatusLive`. |
| `TestMarkPublishedDoesNotPromoteSuspended` | Un suspendido publicado sigue `StatusSuspended`. |
| `TestMarkPublishedRequiresRef` | `ref == ""` → `ErrInvalidData` y **el sitio no cambia**. |

El último importa: comprueba el estado en la base después del error, no sólo el
error. Una validación que ya escribió algo antes de fallar no es una validación.

## 4. Documentación

- [`docs/ARCHITECTURE.md`](ARCHITECTURE.md): añade el ciclo de publicación —
  quién marca sucio (el panel, al guardar), quién consulta la lista (CI) y quién
  marca publicado (CI, al terminar). Deja escrito que `Status` y `Dirty` son
  ejes independientes.
- [`README.md`](../README.md): re-indexa si hace falta y documenta los tres
  métodos nuevos en la sección de API pública, con la firma exacta.

Ningún documento debe citar `docs/PLAN.md`: este archivo se borra al publicar.

## 5. Criterios de aceptación

- [ ] `gotest` en verde.
- [ ] `gofmt -l .` vacío.
- [ ] `grep -rn "MarkDirty\|DirtySites\|MarkPublished" *.go` → sólo `publish.go`.
- [ ] `grep -rn "map\[" *.go` → vacío. Este módulo compila con TinyGo dentro de un Worker.
- [ ] `grep -rn "\"time\"\|\"strings\"\|\"errors\"\|\"fmt\"\|encoding/json" *.go` → vacío: se usan `github.com/tinywasm/time` y `github.com/tinywasm/fmt`.
- [ ] `models.go`, `models_orm.go` y `validTransitions` **sin cambios**: `git diff` no los toca.
- [ ] Los once tests de §3 existen y pasan.

## 6. Anti-footguns

1. **No agregues campos ni modelos.** `Dirty`, `PublishedAt` y `PublishedRef` ya
   existen en `Site` y en `SiteModel`. Este plan sólo escribe métodos.
2. **No toques la tabla de transiciones.** No hace falta una transición hacia
   `Draft`: `Dirty` es lo que expresa "hay cambios sin publicar", y ese fue
   exactamente el error que originó este plan.
3. **Sin stdlib.** Este módulo se compila con TinyGo dentro de un Cloudflare
   Worker con un límite duro de 1 MB, y el panel que lo importa ya va por el
   68 % de ese presupuesto. `fmt` es `github.com/tinywasm/fmt`; `time` es
   `github.com/tinywasm/time`. Sin `map`, sin `reflect`, sin `encoding/json`.
4. **Sin backend concreto fuera de los tests.** La lista blanca de `AGENTS.md`
   manda: el módulo recibe un `*orm.DB` y no sabe qué hay detrás.
5. `docs/PLAN.md` (este archivo) no se renombra ni se borra, y su bloque de
   frontmatter —`PLAN`, `TAG`, `EXECUTOR`, `STATUS`, `SESSION`, `PR`— **no se
   edita a mano**: lo escribe `codejob`.
