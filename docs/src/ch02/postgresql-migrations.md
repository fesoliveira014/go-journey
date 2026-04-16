# 2.2 PostgreSQL & Migrations

The Catalog service needs a database. This section gets PostgreSQL running locally, explains why raw SQL migrations beat ORM magic in production, walks through the actual migration files for the books table, and shows how Go's `embed` package bundles those SQL files directly into the compiled binary.

---

## Running PostgreSQL Locally

The fastest way to get a development PostgreSQL instance is Docker. No installation, no path configuration, no version conflicts — a single command does it:

```bash
docker run -d \
  --name catalog-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=catalog \
  -p 5432:5432 \
  postgres:16
```

Flags worth noting:
- `-d` — detached mode; the container runs in the background
- `--name catalog-postgres` — gives the container a stable name so you can reference it later (`docker stop catalog-postgres`, `docker logs catalog-postgres`, etc.)
- `-e POSTGRES_DB=catalog` — creates the `catalog` database on first boot; without this you'd have to create it manually
- `-p 5432:5432` — maps the container's port 5432 to your host machine's port 5432

To connect with `psql` (the PostgreSQL CLI):

```bash
psql -h localhost -U postgres -d catalog
```

When prompted, enter the password `postgres`. You'll land at the `catalog=#` prompt. Commands worth knowing:

| Command | Description |
|---|---|
| `\dt` | List all tables |
| `\d books` | Describe the `books` table schema |
| `\q` | Quit |
| `SELECT * FROM schema_migrations;` | Inspect migration history |

---

## Why Versioned Migrations?

Before reaching for a migration tool, consider what goes wrong without one.

### The problem with manual schema changes

On a solo project with a single database, the typical flow looks like: connect to the DB, run an `ALTER TABLE`, move on. This breaks down quickly:

- **No shared state**: other developers don't know what changes you made, or in what order. Their local DB drifts from yours.
- **No history**: six months later, you can't tell if a column was always there or added late. There's no `git log` for your schema.
- **No rollback**: if the change is wrong, you need to reverse it manually — and remember exactly what you changed.
- **No environment parity**: the schemas in production, staging, and local dev gradually diverge.

### Why GORM AutoMigrate is dangerous in production

GORM ships a feature called `AutoMigrate` that creates or alters tables to match your Go struct definitions. It sounds convenient and is useful for exploratory development. You should not use it in production for two reasons:

1. **It never drops columns.** If you remove a field from your struct, `AutoMigrate` leaves the column in the database untouched. Over time, the schema accumulates ghost columns that exist in the DB but nowhere in your code. Reads and writes may silently behave differently than you expect.

2. **It has no version tracking.** There's no record of what was applied, when, or in what order. You can't roll back. You can't replay. You can't audit. Every environment drifts silently with no way to tell.

### What golang-migrate gives you

`golang-migrate`[^1] solves this with a simple model: Every schema change is a pair of SQL files — an `up` migration that applies the change and a `down` migration that reverses it. Files are numbered sequentially. The tool tracks which migrations have been applied in a `schema_migrations` table it manages itself.

The result:
- Every environment can replay the full migration history from scratch
- Rolling back a deployment means running the down migration
- The migration history is version-controlled alongside the application code
- The schema state is deterministic and auditable

---

## Writing Migrations

Migration files follow a strict naming convention:

```
{version}_{description}.up.sql
{version}_{description}.down.sql
```

The version is a zero-padded integer (conventionally six digits). The description is a short slug. For the books table, the files are:

```
services/catalog/migrations/000001_create_books.up.sql
services/catalog/migrations/000001_create_books.down.sql
```

### The up migration

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE books (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title            VARCHAR(500) NOT NULL,
    author           VARCHAR(500) NOT NULL,
    isbn             VARCHAR(13) UNIQUE,
    genre            VARCHAR(100),
    description      TEXT,
    published_year   INTEGER,
    total_copies     INTEGER NOT NULL DEFAULT 1,
    available_copies INTEGER NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT available_lte_total CHECK (available_copies <= total_copies),
    CONSTRAINT copies_non_negative CHECK (available_copies >= 0 AND total_copies >= 0)
);

CREATE INDEX idx_books_genre ON books(genre);
CREATE INDEX idx_books_author ON books(author);
```

Several PostgreSQL-specific features are in play here:

**`uuid-ossp` extension and `uuid_generate_v4()`**

PostgreSQL doesn't generate UUIDs out of the box. The `uuid-ossp` extension adds UUID generation functions. `uuid_generate_v4()` produces a random v4 UUID. The `CREATE EXTENSION IF NOT EXISTS` guard makes the migration idempotent — safe to re-run without error if the extension is already loaded.

PostgreSQL 13+ ships `gen_random_uuid()` as a built-in (no extension needed), but `uuid_generate_v4()` is widely used and works on any supported version.

**`TIMESTAMPTZ` (not `TIMESTAMP`)**

`TIMESTAMPTZ` is "timestamp with time zone". Despite the name, PostgreSQL doesn't store the timezone — it stores everything as UTC and converts on read based on the session's `TimeZone` setting. `TIMESTAMP` (without timezone) stores whatever you give it with no conversion. Always use `TIMESTAMPTZ` for application timestamps. It prevents a class of subtle timezone bugs where rows inserted from different regions have timestamps that don't sort correctly.

**CHECK constraints**

The two `CONSTRAINT` lines enforce business rules at the database level:

- `available_lte_total` — available copies can never exceed total copies
- `copies_non_negative` — neither count can go negative

Naming constraints is important. When a constraint is violated, the error message includes its name, which makes debugging from logs much faster. With an auto-generated name you'd see `books_available_copies_total_copies_check` or similar — with a named constraint you see `available_lte_total`, which is self-documenting.

**Indexes**

The two index statements exist because the catalog supports filtering by genre and author. Without indexes, those queries would do a full table scan. For a library with thousands of books, that's acceptable; for one with millions, it isn't. Adding indexes in the migration that creates the table is the right time — you're declaring "this column will be queried" alongside the schema definition.[^2]

### The down migration

```sql
DROP TABLE IF EXISTS books;
```

Down migrations should undo the up migration completely. Since the up migration creates the `books` table (and the `uuid-ossp` extension), the down migration drops it. We don't reverse the extension because other tables might depend on it.

Down migrations are most valuable for rolling back a bad deployment. The workflow is: deploy new code → something is wrong → run down migration → redeploy previous version. This only works if your down migrations are correct and kept up to date with the up migrations.

---

## Embedding Migrations in Go

The SQL files live at `services/catalog/migrations/`. They need to be accessible at runtime when the service starts. There are two approaches:

1. **External files** — deploy the SQL files alongside the binary and read them from the filesystem at runtime
2. **Embedded files** — compile the SQL files into the binary itself

We use the second approach. The entire `migrations/` package is:

```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

The `//go:embed *.sql` directive is a compiler instruction.[^3] It tells the Go compiler to find all files matching `*.sql` relative to this source file and embed them into the compiled binary as a virtual filesystem. The `embed.FS` type (from the `embed` package, introduced in Go 1.16) exposes a standard `fs.FS` interface for reading those embedded files.

Why embed rather than ship external files?

- **Single binary deployment**: your container image, Lambda function, or VM only needs one artifact. There's no "oops, I forgot to copy the migrations directory" class of deployment failures.
- **Immutability**: the migrations bundled with a given binary version are fixed. You can't accidentally run the wrong migrations against a database by deploying mismatched files.
- **Simpler CI/CD**: one binary to test, one binary to ship, one binary to run.

The trade-off is that you can't add or modify migrations without recompiling. For database migrations, that's not a trade-off — migrations should be immutable once deployed and always version-controlled with the code that depends on them.

The next section shows how `runMigrations()` consumes this embedded filesystem.

---

## Running Migrations Programmatically

The `runMigrations()` function in `services/catalog/cmd/main.go` applies all pending migrations when the service starts:

```go
func runMigrations(db *gorm.DB) error {
    sqlDB, err := db.DB()
    if err != nil {
        return fmt.Errorf("get sql.DB: %w", err)
    }

    driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
    if err != nil {
        return fmt.Errorf("create migration driver: %w", err)
    }

    source, err := iofs.New(migrations.FS, ".")
    if err != nil {
        return fmt.Errorf("create migration source: %w", err)
    }

    m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
    if err != nil {
        return fmt.Errorf("create migrator: %w", err)
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("run migrations: %w", err)
    }

    return nil
}
```

Walking through each step:

1. **`db.DB()`** — GORM wraps the standard `*sql.DB`. `golang-migrate` works with `*sql.DB` directly, so we unwrap it.

2. **`pgmigrate.WithInstance`** — creates a PostgreSQL migration driver from the connection. This driver manages the `schema_migrations` table.

3. **`iofs.New(migrations.FS, ".")`** — creates a migration source from the embedded filesystem. The `"."` argument is the root directory inside the embedded filesystem to search for migration files. This is where the `embed.FS` from the `migrations` package gets handed to golang-migrate.

4. **`migrate.NewWithInstance`** — wires the source and driver together. The string arguments (`"iofs"`, `"postgres"`) are driver names used internally.

5. **`m.Up()`** — applies all unapplied migrations in version order. The critical line is `err != migrate.ErrNoChange`: if no new migrations exist, `m.Up()` returns `migrate.ErrNoChange` rather than `nil`. Treating `ErrNoChange` as an error would cause the service to crash on every startup after the first deployment. We treat it as success.

This pattern — running migrations automatically on service startup — is appropriate for a microservices environment where each service owns its database schema. The service doesn't start serving traffic until migrations have completed.

---

## Exercise

Write a second migration that adds a `language` column to the `books` table.

**Requirements:**
- File: `services/catalog/migrations/000002_add_language_column.up.sql`
- Column: `language VARCHAR(50)` with a default of `'English'`
- Write the corresponding down migration: `000002_add_language_column.down.sql`
- Restart the Catalog Service (or run `psql` commands manually) and verify that:
  - After running the up migration, `\d books` shows the new `language` column
  - After running the down migration, the column is gone

<details>
<summary>Solution</summary>

**`000002_add_language_column.up.sql`**

```sql
ALTER TABLE books ADD COLUMN language VARCHAR(50) NOT NULL DEFAULT 'English';
```

A few choices made here:
- `NOT NULL` is safe because we supply a `DEFAULT`. All existing rows will have `'English'` populated immediately when the migration runs.
- Adding `DEFAULT` in the same statement as `ADD COLUMN` is an atomic operation in PostgreSQL 11+. The server fills in the default value for existing rows during the `ALTER TABLE` rather than requiring a separate `UPDATE`. On older versions this could be a two-step process; on modern PostgreSQL it's one statement.

**`000002_add_language_column.down.sql`**

```sql
ALTER TABLE books DROP COLUMN IF EXISTS language;
```

`IF EXISTS` makes the down migration idempotent. If someone runs it twice (or the column was never added), it doesn't error.

**Verifying in psql:**

```bash
# Connect
psql -h localhost -U postgres -d catalog

# After running up migration:
\d books
# Should show:  language | character varying(50) | not null default 'English'

# After running down migration:
\d books
# language column should be absent
```

If you're running the Catalog Service locally, stop it, apply the migration, and restart. The `runMigrations()` call at startup will pick up `000002` since it's higher than the last applied version recorded in `schema_migrations`.

</details>

---

## Summary

- Run a development PostgreSQL instance with a single `docker run` command; connect with `psql` for inspection
- `AutoMigrate` is useful in development but dangerous in production: it never drops columns and tracks no history
- `golang-migrate` provides ordered, reversible, version-tracked SQL migrations via paired `.up.sql` / `.down.sql` files
- PostgreSQL-specific features in the books schema: `uuid-ossp` for UUID generation, `TIMESTAMPTZ` for timezone-correct timestamps, named `CHECK` constraints for data integrity, and indexes on frequently-queried columns
- `//go:embed *.sql` compiles SQL files directly into the binary — no separate file deployment required
- `runMigrations()` runs at startup; `migrate.ErrNoChange` is expected on every run after the first and must be treated as success

---

## References

[^1]: [golang-migrate documentation](https://github.com/golang-migrate/migrate)
[^2]: [PostgreSQL CREATE TABLE](https://www.postgresql.org/docs/current/sql-createtable.html)
[^3]: [Go embed package](https://pkg.go.dev/embed)
