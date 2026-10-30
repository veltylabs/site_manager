---
PLAN: "fix: migrate with ddl.Sync so a new field reaches an existing table"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 9024087468757363535
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work (a previous run of this plan
> was lost exactly that way).

# Migrate with `ddl.Sync`

## Context

`New` (in `module.go`) creates this repo's tables with `ddl.DB.CreateTable`, which compiles to `CREATE TABLE IF NOT EXISTS`:
against a table that already exists it does nothing. So when a model gains a field, a database that is already
installed never receives the new column, and the first read or write of that field fails in production.
`ddl.DB.Sync` creates the missing table **and** adds the missing columns (it also renames a column the model declares
as renamed, and drops a column only when it holds no data). Its flow, branch by branch, is this diagram — the tests
below walk its branches: https://github.com/webtyp/ddl/blob/main/docs/diagrams/SYNC_FLOW.md

Rule decided for the whole ecosystem: nobody writes migration files; every library and module migrates with `Sync`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/ddl@v0.0.36
go mod tidy
```

Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.

## APIs used

| Symbol | Exact signature | Tag |
|---|---|---|
| `ddl.New` | `func New(conn Execer, ddlCompiler Compiler) *DB` | `webtyp.com/ddl` v0.0.36 |
| `(*ddl.DB).Sync` | `func (d *DB) Sync(models ...model.Model) error` — models in the order given; a connection without transactions syncs model by model | v0.0.36 |
| `ddl.Execer` | `interface { Exec(query string, args ...any) error }` | v0.0.36 |
| `ddl.Compiler` | `interface { CompileDDL(s Stmt, m model.Model) (query string, args []any, err error) }` | v0.0.36 |
| `ddl.TableIntrospector` | `interface { TableColumns(table string) ([]string, error) }` — when the connection has it, `Sync` adds only the missing columns; when `TableColumns` fails, `Sync` returns that same error and touches no later model | v0.0.36 |
| `ddl.Stmt` | `struct { Op Op; Table string; Database string; Column *model.Field; OldName string; ColumnName string }` | v0.0.36 |
| `ddl.OpCreateTable`, `ddl.OpAddColumn` | constants of type `ddl.Op` | v0.0.36 |
| `model.Model` | has `ModelName() string` and `Schema() []model.Field`; `model.Field.Name string` | `webtyp.com/model` (already required) |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — tests first: `tests/migrate_test.go` (new file)

Use the package name the other files in `tests/` already use (`package tests` if the folder is new). The tests are
written **before** the code: run `gotest` after this stage and see `TestMigrate_NewField_AddsOnlyThatColumn` fail
(today the migration never adds a column). Write exactly this file:

```go
package tests

import (
	"errors"
	"testing"

	"webtyp.com/ddl"
	"webtyp.com/model"

	sitemanager "github.com/veltylabs/site_manager"
)

// migrateConn is the database the migration sees: Exec does nothing, and
// TableColumns answers which columns each table already has.
type migrateConn struct {
	existing  []tableColumns
	failTable string
}

type tableColumns struct {
	table   string
	columns []string
}

var errIntrospect = errors.New("introspection failed")

func (c *migrateConn) Exec(string, ...any) error { return nil }

func (c *migrateConn) TableColumns(table string) ([]string, error) {
	if table == c.failTable {
		return nil, errIntrospect
	}
	for _, t := range c.existing {
		if t.table == table {
			return t.columns, nil
		}
	}
	return nil, nil
}

// recordingCompiler keeps, in order, every statement the migration asked for.
type recordingCompiler struct{ stmts []ddl.Stmt }

func (r *recordingCompiler) CompileDDL(s ddl.Stmt, _ model.Model) (string, []any, error) {
	r.stmts = append(r.stmts, s)
	return "", nil, nil
}

// migrationModels are this repo's tables in foreign-key order: the order Migrate uses.
func migrationModels() []model.Model {
	return []model.Model{&sitemanager.Site{}, &sitemanager.SiteMember{}, &sitemanager.Plan{}, &sitemanager.AccessRequest{}}
}

func columnsOf(m model.Model) []string {
	var out []string
	for _, f := range m.Schema() {
		out = append(out, f.Name)
	}
	return out
}

func upToDate() []tableColumns {
	var out []tableColumns
	for _, m := range migrationModels() {
		out = append(out, tableColumns{table: m.ModelName(), columns: columnsOf(m)})
	}
	return out
}

// Branch "listing works, nothing missing": only the idempotent CREATE TABLE per table, in order.
func TestMigrate_UpToDate_OnlyEnsuresTables(t *testing.T) {
	comp := &recordingCompiler{}
	if err := sitemanager.Migrate(&migrateConn{existing: upToDate()}, comp); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	models := migrationModels()
	if len(comp.stmts) != len(models) {
		t.Fatalf("got %d statements, want %d: %+v", len(comp.stmts), len(models), comp.stmts)
	}
	for i, m := range models {
		if comp.stmts[i].Op != ddl.OpCreateTable || comp.stmts[i].Table != m.ModelName() {
			t.Errorf("statement %d: got %+v, want CREATE TABLE %s (foreign-key order)", i, comp.stmts[i], m.ModelName())
		}
	}
}

// Branch "field missing from the table": exactly one ADD COLUMN, for that field of that table.
func TestMigrate_NewField_AddsOnlyThatColumn(t *testing.T) {
	models := migrationModels()
	last := models[len(models)-1]
	cols := columnsOf(last)
	existing := upToDate()
	existing[len(existing)-1].columns = cols[:len(cols)-1]

	comp := &recordingCompiler{}
	if err := sitemanager.Migrate(&migrateConn{existing: existing}, comp); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var adds []ddl.Stmt
	for _, s := range comp.stmts {
		if s.Op == ddl.OpAddColumn {
			adds = append(adds, s)
		}
	}
	if len(adds) != 1 {
		t.Fatalf("got %d ADD COLUMN, want 1: %+v", len(adds), adds)
	}
	if adds[0].Table != last.ModelName() || adds[0].Column == nil || adds[0].Column.Name != cols[len(cols)-1] {
		t.Fatalf("got %+v, want ADD COLUMN %s.%s", adds[0], last.ModelName(), cols[len(cols)-1])
	}
}

// Branch "listing the columns failed": the error comes back and no later table is touched.
func TestMigrate_IntrospectionFails_ReturnsErrorAndStops(t *testing.T) {
	first := migrationModels()[0]
	comp := &recordingCompiler{}
	err := sitemanager.Migrate(&migrateConn{existing: upToDate(), failTable: first.ModelName()}, comp)
	if !errors.Is(err, errIntrospect) {
		t.Fatalf("got %v, want the introspection error", err)
	}
	for _, s := range comp.stmts {
		if s.Table != first.ModelName() {
			t.Fatalf("touched %s after the failure on %s", s.Table, first.ModelName())
		}
	}
}
```


Commit after this stage (the red test is expected here).

## Stage 2 — `migrate.go` (new, repo root) and `module.go`

Create `migrate.go` in the root package (`package sitemanager`):

```go
package sitemanager

import "webtyp.com/ddl"

// Migrate crea o pone al día las tablas de este módulo. Usa Sync y no CreateTable: crea la tabla
// que falta y además agrega las columnas que un modelo ganó, así un campo nuevo llega a una base ya
// instalada. Flujo: https://github.com/webtyp/ddl/blob/main/docs/diagrams/SYNC_FLOW.md
//
// Ejemplo: err := sitemanager.Migrate(conn, compiler)
func Migrate(conn ddl.Execer, ddlCompiler ddl.Compiler) error {
	return ddl.New(conn, ddlCompiler).Sync(
		&Site{},
		&SiteMember{},
		&Plan{},
		&AccessRequest{},
	)
}
```

In `module.go`, `New` keeps migrating (consumers rely on it; do not change its signature or its other checks). Replace
the whole `if …, ok := d.DB.RawConn().(ddl.Compiler); ok { … CreateTable … }` block with:

```go
	if c, ok := d.DB.RawConn().(ddl.Compiler); ok {
		if err := Migrate(d.DB.RawConn(), c); err != nil {
			return nil, err
		}
	}
```

Run `gotest`: the three tests and the existing ones pass. Commit after this stage.

## Stage 3 — docs

If `README.md` or any `docs/*.md` says the tables are created with `CreateTable` (or "creates the tables" without
mentioning that new columns are added), change it to say that `New` migrates through `sitemanager.Migrate`, which uses `ddl.Sync`, en español, and
link the diagram https://github.com/webtyp/ddl/blob/main/docs/diagrams/SYNC_FLOW.md. If no doc mentions the migration, add one line to `README.md` under its usage or
installation section saying so. Commit after this stage.

## Stage 4 — language

Translate to Spanish every `README.md`, `docs/*` file, diagram and code comment of this repo that is still in English
(only those; never rewrite what is already Spanish), and add a short usage example (`// Ejemplo: …`) to every exported
symbol's comment that lacks one. Code, identifiers and translation keys stay in English. Commit after this stage.

## Delivery check

1. `gotest` passes (it builds, vets and runs every test). A PR that does not compile, or whose tests could not run, is a failed delivery.
2. `grep -rn "CreateTable(" --include=*.go . | grep -v _test.go` → empty.
3. `git diff --stat main` lists `tests/migrate_test.go`, `migrate.go`, `module.go`, `go.mod`, `go.sum`, the docs touched — nothing else deleted.

## Delivery rules

- Never pause to ask permission or report progress: when the delivery check passes, commit, push and open the PR. Stuck
  for 30 minutes: open the PR anyway, listing what fails.
- Commit after every stage. Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
- Never edit a generated file (`model_orm.go`): a build error there means the hand-written code is wrong.
- Do not change the models, their order, or any other package.

## Stages

| # | Files | Result |
|---|---|---|
| 1 | `tests/migrate_test.go` | Three tests, one per branch; the new-field test is red |
| 2 | `migrate.go`, `module.go` | `Migrate` uses `Sync`, `New` calls it; all green |
| 3 | `README.md` / `docs/*` | The migration is documented with the diagram |
| 4 | every `README.md`, `docs/*`, comment still in English | Spanish |

## Executor notes
- **Stage 1**: Created `tests/migrate_test.go` and implemented the three migration tests successfully. The tests cover the `UpToDate`, `NewField`, and `IntrospectionFails` branches exactly as requested.
- **Stage 2**: Created `migrate.go` at the repo root and successfully refactored `module.go`'s `New` method to use the new `Migrate` function passing in a single `ddl.Sync` run, instead of four distinct `CreateTable` runs. Tests pass.
- **Stage 3**: Added the Spanish explanation for `New`'s usage of `sitemanager.Migrate` and `ddl.Sync` along with the diagram link to the "Inicio Rápido" usage section in `README.md`.
- **Stage 4**: Thoroughly updated English comments and added `// Ejemplo:` usage examples across all major files (`access.go`, `errors.go`, `member.go`, `module.go`, `plan.go`, `publish.go`, and `site.go`). Checked static analysis targets (`go vet`).
- **Delivery Check**: `gotest` run was green. `grep` check for `CreateTable(` turned up empty, and the final list of modified files was as expected.
