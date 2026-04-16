# 2.2 PostgreSQL & Migrations

<!-- [STRUCTURAL] Opening paragraph lays out four concrete promises (docker, why raw SQL, walkthrough, embed). That is a lot — consider promising fewer things and over-delivering, or split the last two into a "we will" list for scannability. -->
The Catalog service needs a database. This section gets PostgreSQL running locally, explains why raw SQL migrations beat ORM magic in production, walks through the actual migration files for the books table, and shows how Go's `embed` package bundles those SQL files directly into the compiled binary.

---

## Running PostgreSQL Locally

<!-- [LINE EDIT] "The fastest way to get a development PostgreSQL instance is Docker. No installation, no path configuration, no version conflicts — just one command:" — the "just one command" contains the banned "just". Suggest: "No installation, no path configuration, no version conflicts — a single command does it:" -->
The fastest way to get a development PostgreSQL instance is Docker. No installation, no path configuration, no version conflicts — just one command:

```bash
docker run -d \
  --name catalog-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=catalog \
  -p 5432:5432 \
  postgres:16
```

<!-- [COPY EDIT] "postgres:16" — Please verify: is 16 the intended major version (17 is current as of late 2024/early 2025)? If the book is to age, consider "postgres:16" with a footnote, or bump to latest stable. -->
Flags worth noting:
- `-d` — detached mode; the container runs in the background
- `--name catalog-postgres` — gives the container a stable name so you can reference it later (`docker stop catalog-postgres`, `docker logs catalog-postgres`, etc.)
- `-e POSTGRES_DB=catalog` — creates the `catalog` database on first boot; without this you'd have to create it manually
- `-p 5432:5432` — maps the container's port 5432 to your host machine's port 5432
<!-- [COPY EDIT] "5432" — unitless port numbers are always numerals (CMOS 9.2). Good. -->

<!-- [LINE EDIT] "To connect with `psql` (the PostgreSQL CLI):" — fine; leave. -->
To connect with `psql` (the PostgreSQL CLI):

```bash
psql -h localhost -U postgres -d catalog
```

<!-- [LINE EDIT] "You'll land at the `catalog=#` prompt." — "land at" is conversational; fine. "Useful commands to remember:" → "Commands worth knowing:" is slightly less filler. -->
When prompted, enter the password `postgres`. You'll land at the `catalog=#` prompt. Useful commands to remember:

| Command | Description |
|---|---|
| `\dt` | List all tables |
| `\d books` | Describe the `books` table schema |
| `\q` | Quit |
| `SELECT * FROM schema_migrations;` | Inspect migration history |

---

## Why Versioned Migrations?

<!-- [STRUCTURAL] This is the section's pedagogical heart: you ground the why-migrations argument in concrete failure modes before introducing the tool. Strong. -->
Before reaching for a migration tool, it's worth understanding what goes wrong without one.

### The problem with manual schema changes

<!-- [COPY EDIT] Heading case inconsistency: "Why Versioned Migrations?" uses title case; "The problem with manual schema changes" uses sentence case. Other H3s in this file ("The up migration", "The down migration") are sentence case; "Why GORM AutoMigrate is dangerous in production" is sentence case; "What golang-migrate gives you" is sentence case; "Writing Migrations" is title case. Recommend standardizing to sentence case for H3s, title case for H2s — then fix "Writing Migrations" heading to match. -->
<!-- [LINE EDIT] "On a solo project with a single database, the typical flow looks like: connect to the DB, run an `ALTER TABLE`, move on." — active and vivid. Leave. -->
On a solo project with a single database, the typical flow looks like: connect to the DB, run an `ALTER TABLE`, move on. This breaks down quickly:

- **No shared state**: other developers don't know what changes you made, or in what order. Their local DB drifts from yours.
- **No history**: six months later, you can't tell if a column was always there or added late. There's no `git log` for your schema.
- **No rollback**: if the change is wrong, you need to reverse it manually — and remember exactly what you changed.
<!-- [COPY EDIT] "local dev gradually diverge" — subject–verb: "schema ... diverge" (plural agreement with compound subject "schema in production, staging, and local dev") — acceptable but awkward because "schema" is treated as a collective. Consider "the schemas in production, staging, and local dev gradually diverge." -->
- **No environment parity**: the schema in production, staging, and local dev gradually diverge.

### Why GORM AutoMigrate is dangerous in production

<!-- [STRUCTURAL] The `AutoMigrate` critique lands at the right place — readers familiar with JPA/Hibernate's auto-DDL will have been eyeing this. Explicit "do not use it in production" is worth the emphasis. -->
GORM ships a feature called `AutoMigrate` that creates or alters tables to match your Go struct definitions. It sounds convenient and is genuinely useful for exploratory development. You should not use it in production for two reasons:

1. **It never drops columns.** If you remove a field from your struct, `AutoMigrate` leaves the column in the database untouched. Over time, the schema accumulates ghost columns that exist in the DB but nowhere in your code. Reads and writes may silently behave differently than you expect.

<!-- [LINE EDIT] "Every environment is potentially in a different state with no way to tell." — adverb-heavy; tighten: "Every environment drifts silently with no way to tell." -->
2. **It has no version tracking.** There's no record of what was applied, when, or in what order. You can't roll back. You can't replay. You can't audit. Every environment is potentially in a different state with no way to tell.

### What golang-migrate gives you

<!-- [COPY EDIT] "`golang-migrate`[^1] solves this with a simple model" — footnote placement inside backticks should move outside: "`golang-migrate`[^1]" is already outside; good. -->
`golang-migrate`[^1] solves this with a simple model: every schema change is a pair of SQL files — an `up` migration that applies the change and a `down` migration that reverses it. Files are numbered sequentially. The tool tracks which migrations have been applied in a `schema_migrations` table it manages itself.

The result:
- Every environment can replay the full migration history from scratch
- Rolling back a deployment means running the down migration
- The migration history is version-controlled alongside the application code
- The schema state is deterministic and auditable

---

## Writing Migrations

<!-- [COPY EDIT] Heading case: "Writing Migrations" title case vs sentence-case H3s elsewhere. See earlier note — pick one convention and apply consistently across file. -->
Migration files follow a strict naming convention:

```
{version}_{description}.up.sql
{version}_{description}.down.sql
```

<!-- [LINE EDIT] "The version is a zero-padded integer (conventionally six digits)." — fine. Leave. -->
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

<!-- [STRUCTURAL] The inline rationale for each PG feature (uuid-ossp, TIMESTAMPTZ, CHECK, indexes) is well-targeted for a Java/Kotlin dev whose PostgreSQL reflexes may differ. Keep the structure. -->
Several PostgreSQL-specific features are in play here:

**`uuid-ossp` extension and `uuid_generate_v4()`**

<!-- [LINE EDIT] "PostgreSQL doesn't generate UUIDs out of the box." — fine. "The `CREATE EXTENSION IF NOT EXISTS` guard makes the migration idempotent — safe to re-run without error if the extension is already loaded." — fine. -->
PostgreSQL doesn't generate UUIDs out of the box. The `uuid-ossp` extension adds UUID generation functions. `uuid_generate_v4()` produces a random v4 UUID. The `CREATE EXTENSION IF NOT EXISTS` guard makes the migration idempotent — safe to re-run without error if the extension is already loaded.

<!-- [COPY EDIT] "PostgreSQL 13+" — verify: as of the book's target date (2026), PG 17 is GA. Consider rephrasing: "PostgreSQL 13 and later ship `gen_random_uuid()` as a built-in..." for broader forward-compatibility reading. -->
<!-- [STRUCTURAL] Consider briefly recommending `gen_random_uuid()` as the preferred choice for new projects on PG 13+, noting that the book uses `uuid_generate_v4()` only for broader version support. Right now it reads as if the uuid-ossp route is recommended without caveat. -->
PostgreSQL 13+ ships `gen_random_uuid()` as a built-in (no extension needed), but `uuid_generate_v4()` is widely used and works on any supported version.

**`TIMESTAMPTZ` (not `TIMESTAMP`)**

<!-- [LINE EDIT] "Despite the name, PostgreSQL doesn't store the timezone — it stores everything as UTC and converts on read based on the session's `TimeZone` setting." — good; leave. -->
<!-- [LINE EDIT] "Always use `TIMESTAMPTZ` for application timestamps. It prevents a class of subtle timezone bugs where rows inserted from different regions have timestamps that don't sort correctly." — solid rule-of-thumb statement. -->
`TIMESTAMPTZ` is "timestamp with time zone". Despite the name, PostgreSQL doesn't store the timezone — it stores everything as UTC and converts on read based on the session's `TimeZone` setting. `TIMESTAMP` (without timezone) stores whatever you give it with no conversion. Always use `TIMESTAMPTZ` for application timestamps. It prevents a class of subtle timezone bugs where rows inserted from different regions have timestamps that don't sort correctly.

**CHECK constraints**

The two `CONSTRAINT` lines enforce business rules at the database level:

- `available_lte_total` — available copies can never exceed total copies
- `copies_non_negative` — neither count can go negative

<!-- [LINE EDIT] "Naming constraints (rather than letting PostgreSQL auto-generate a name) is important." — fine. The long mid-sentence "you'd see" example is a mild readability hit; consider breaking: "Name your constraints explicitly. When a constraint is violated, the error message includes its name — a lifesaver when debugging from logs. PostgreSQL's auto-generated names like `books_available_copies_total_copies_check` are verbose and opaque; `available_lte_total` is self-documenting." -->
Naming constraints (rather than letting PostgreSQL auto-generate a name) is important. When a constraint is violated, the error message includes its name, which makes debugging from logs much faster. With an auto-generated name you'd see `books_available_copies_total_copies_check` or similar — with a named constraint you see `available_lte_total`, which is self-documenting.

**Indexes**

<!-- [LINE EDIT] "`CREATE INDEX idx_books_genre ON books(genre)` and `idx_books_author ON books(author)` exist because..." — second conjunct drops `CREATE INDEX` prefix; inconsistent. Consider: "The two index statements exist because..." -->
<!-- [LINE EDIT] "For a library with thousands of books, that's acceptable; for one with millions, it isn't." — compact and effective. Leave. -->
`CREATE INDEX idx_books_genre ON books(genre)` and `idx_books_author ON books(author)` exist because the catalog supports filtering by genre and author. Without indexes, those queries would do a full table scan. For a library with thousands of books, that's acceptable; for one with millions, it isn't. Adding indexes in the migration that creates the table is the right time — you're declaring "this column will be queried" alongside the schema definition.[^2]

### The down migration

```sql
DROP TABLE IF EXISTS books;
```

<!-- [LINE EDIT] "Down migrations should undo the up migration completely. Since the up migration creates the `books` table (and the `uuid-ossp` extension), the down migration drops it. We don't reverse the extension because other tables might depend on it." — fine, but note the asymmetry pedagogically: the extension is one-way even though "completely" is the stated goal. Consider: "Down migrations should undo the up migration — with a caveat. We drop the `books` table, but we leave the `uuid-ossp` extension in place, since other tables (added by later migrations) may depend on it." -->
Down migrations should undo the up migration completely. Since the up migration creates the `books` table (and the `uuid-ossp` extension), the down migration drops it. We don't reverse the extension because other tables might depend on it.

<!-- [STRUCTURAL] The "when down migrations are valuable" paragraph is pragmatic and honest. Some books skip this nuance; yours doesn't. Good. -->
Down migrations are most valuable for rolling back a bad deployment. The workflow is: deploy new code → something is wrong → run down migration → redeploy previous version. This only works if your down migrations are correct and kept up to date with the up migrations.

---

## Embedding Migrations in Go

<!-- [STRUCTURAL] The two-option framing (external files vs embedded) before choosing the second is pedagogically strong — it gives the reader a mental model even if they only see one in the code. -->
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

<!-- [LINE EDIT] "The `//go:embed *.sql` directive is a compiler instruction.[^3] It tells the Go compiler to find all files matching `*.sql` relative to this source file and embed them into the compiled binary as a virtual filesystem." — sentence is 36 words; still readable. Consider splitting: "The `//go:embed *.sql` directive is a compiler instruction.[^3] It tells the compiler to find files matching `*.sql` relative to this source file and embed them into the binary as a virtual filesystem." -->
The `//go:embed *.sql` directive is a compiler instruction.[^3] It tells the Go compiler to find all files matching `*.sql` relative to this source file and embed them into the compiled binary as a virtual filesystem. The `embed.FS` type (from the `embed` package, new in Go 1.16) exposes a standard `fs.FS` interface for reading those embedded files.

<!-- [COPY EDIT] "new in Go 1.16" — Go 1.16 shipped in Feb 2021; "new in" is temporally stale for a 2026 book. Suggest: "introduced in Go 1.16" (tense-neutral). -->

Why embed rather than ship external files?

<!-- [LINE EDIT] "your container image, Lambda function, or VM only needs one artifact. There's no 'oops, I forgot to copy the migrations directory' class of deployment failures." — the "oops" is character and should stay. -->
- **Single binary deployment**: your container image, Lambda function, or VM only needs one artifact. There's no "oops, I forgot to copy the migrations directory" class of deployment failures.
- **Immutability**: the migrations bundled with a given binary version are fixed. You can't accidentally run the wrong migrations against a database by deploying mismatched files.
- **Simpler CI/CD**: one binary to test, one binary to ship, one binary to run.

<!-- [LINE EDIT] "The tradeoff is that you can't add or modify migrations without recompiling. For database migrations, that's not a tradeoff — migrations should be immutable once deployed and always version-controlled with the code that depends on them." — good rhetorical pivot ("not a tradeoff"). Leave. -->
The tradeoff is that you can't add or modify migrations without recompiling. For database migrations, that's not a tradeoff — migrations should be immutable once deployed and always version-controlled with the code that depends on them.

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

<!-- [COPY EDIT] Line 211 in source: "err != migrate.ErrNoChange" uses != for error comparison. Idiomatic modern Go prefers `errors.Is(err, migrate.ErrNoChange)` to handle wrapped errors. Please verify: the `migrate.Up()` return does not wrap `ErrNoChange` in current golang-migrate versions, so direct comparison is correct here — but worth a query and/or a sentence acknowledging the idiomatic alternative. -->
<!-- [STRUCTURAL] Numbered walk-through below is excellent — mirrors line-by-line without duplicating code. Keep. -->
Walking through each step:

1. **`db.DB()`** — GORM wraps the standard `*sql.DB`. `golang-migrate` works with `*sql.DB` directly, so we unwrap it.

2. **`pgmigrate.WithInstance`** — creates a PostgreSQL migration driver from the connection. This driver manages the `schema_migrations` table.

<!-- [LINE EDIT] "creates a migration source from the embedded filesystem. The `"."` argument is the root directory within the embedded FS to search for migration files." — "within the embedded FS" is fine but read a bit jargony; consider "the root directory inside the embedded filesystem where migration files live." -->
3. **`iofs.New(migrations.FS, ".")`** — creates a migration source from the embedded filesystem. The `"."` argument is the root directory within the embedded FS to search for migration files. This is where the `embed.FS` from the `migrations` package gets handed to golang-migrate.

4. **`migrate.NewWithInstance`** — wires the source and driver together. The string arguments (`"iofs"`, `"postgres"`) are driver names used internally.

<!-- [LINE EDIT] Sentence > 40 words in step 5: "The critical line is `err != migrate.ErrNoChange`: if no new migrations exist, `m.Up()` returns `migrate.ErrNoChange` rather than `nil`. Treating `ErrNoChange` as an error would cause the service to crash on every startup after the first deployment. We treat it as success." — break into two short sentences already. Actually this is three sentences — fine. Leave but consider replacing "!=" with "errors.Is" idiom. -->
5. **`m.Up()`** — applies all unapplied migrations in version order. The critical line is `err != migrate.ErrNoChange`: if no new migrations exist, `m.Up()` returns `migrate.ErrNoChange` rather than `nil`. Treating `ErrNoChange` as an error would cause the service to crash on every startup after the first deployment. We treat it as success.

<!-- [LINE EDIT] "This pattern — running migrations automatically on service startup — is appropriate for a microservices environment where each service owns its database schema. The service doesn't start serving traffic until migrations have completed." — fine. Consider adding a one-line caveat that startup migrations are not ideal at scale (concurrent replicas racing) and mention advisory locks or init containers — that discussion may belong in a later chapter; a footnote pointer would suffice. -->
<!-- [STRUCTURAL] Consider a sidebar: "Note on startup migrations with multiple replicas" — at scale, N replicas all race to run m.Up(). golang-migrate handles this with advisory locks, which is worth mentioning even briefly for an experienced reader who will immediately ask. -->
This pattern — running migrations automatically on service startup — is appropriate for a microservices environment where each service owns its database schema. The service doesn't start serving traffic until migrations have completed.

---

## Exercise

<!-- [STRUCTURAL] Exercise has clear requirements, explicit file paths, and a verification plan. Strong. -->
Write a second migration that adds a `language` column to the `books` table.

**Requirements:**
- File: `services/catalog/migrations/000002_add_language_column.up.sql`
- Column: `language VARCHAR(50)` with a default of `'English'`
- Write the corresponding down migration: `000002_add_language_column.down.sql`
- Restart the catalog service (or run `psql` commands manually) and verify that:
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
<!-- [COPY EDIT] "PostgreSQL 11+" — current as of the book's date. PG 11 is actually out of community support (EOL Nov 2023); consider "all supported PostgreSQL versions (11 and later)" is still technically correct but you might prefer "all current PostgreSQL versions" for simplicity. -->
- Adding `DEFAULT` in the same statement as `ADD COLUMN` is an atomic operation in PostgreSQL 11+. The server fills in the default value for existing rows during the `ALTER TABLE` rather than requiring a separate `UPDATE`. On older versions this could be a two-step process; on modern PostgreSQL it's one statement.

**`000002_add_language_column.down.sql`**

```sql
ALTER TABLE books DROP COLUMN IF EXISTS language;
```

<!-- [LINE EDIT] "If someone runs it twice (or the column was never added), it doesn't error." — "it doesn't error" is informal but widely accepted in technical prose. Fine. -->
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

<!-- [LINE EDIT] "The `runMigrations()` call at startup will pick up `000002` since it's higher than the last applied version recorded in `schema_migrations`." — fine. -->
If you're running the catalog service locally, stop it, apply the migration, and restart. The `runMigrations()` call at startup will pick up `000002` since it's higher than the last applied version recorded in `schema_migrations`.

</details>

---

## Summary

<!-- [STRUCTURAL] Six summary bullets align with the section's major beats. Good density. -->
- Run a development PostgreSQL instance with a single `docker run` command; connect with `psql` for inspection
- `AutoMigrate` is useful in development but dangerous in production: it never drops columns and tracks no history
- `golang-migrate` provides ordered, reversible, version-tracked SQL migrations via paired `.up.sql` / `.down.sql` files
- PostgreSQL-specific features in the books schema: `uuid-ossp` for UUID generation, `TIMESTAMPTZ` for timezone-correct timestamps, named `CHECK` constraints for data integrity, and indexes on frequently-queried columns
<!-- [COPY EDIT] "frequently-queried columns" — hyphenated compound modifier before noun (CMOS 7.81). Correct. -->
- `//go:embed *.sql` compiles SQL files directly into the binary — no separate file deployment required
- `runMigrations()` runs at startup; `migrate.ErrNoChange` is expected on every run after the first and must be treated as success

---

## References

[^1]: [golang-migrate documentation](https://github.com/golang-migrate/migrate)
[^2]: [PostgreSQL CREATE TABLE](https://www.postgresql.org/docs/current/sql-createtable.html)
[^3]: [Go embed package](https://pkg.go.dev/embed)
<!-- [COPY EDIT] Please verify all three footnote URLs resolve and are canonical. -->
