# 6.1 Admin CLI

<!-- [STRUCTURAL] Section follows a clean problem → design → code → usage → takeaways arc. Good pedagogical rhythm. One structural gap: the jump from "Flag Parsing" to "Database Connection" skips the actual `pkgdb` import that appears out of nowhere on line 80. The imports block earlier shows only GORM + bcrypt; the `pkgdb` helper is never imported in the quoted code. Either show the full import block, add a short note ("we also import `pkg/db`, the project's connection-pool helper — see Chapter 2"), or use `gorm.io/driver/postgres` directly here and simplify. -->

## The Problem

<!-- [LINE EDIT] Tight opener; no changes. -->
The auth service's `Register` RPC always creates users with the `"user"` role. This is the correct default -- you do not want new sign-ups to be admins. But it means there is no way to create the first admin account through the application itself. You could run raw SQL:
<!-- [COPY EDIT] "sign-ups" — CMOS 7.89 / M-W: "sign-up" is the noun (hyphenated). Correct as written. -->
<!-- [COPY EDIT] em dash: `default --` → `default —` (CMOS 6.85). Apply chapter-wide. -->

```sql
UPDATE users SET role = 'admin' WHERE email = 'admin@example.com';
```

<!-- [LINE EDIT] "That works, but it is fragile. You need to know the table and column names, you need to remember to hash the password if you are inserting a new row, and there is no validation." — well-paced; keep. -->
That works, but it is fragile. You need to know the table and column names, you need to remember to hash the password if you are inserting a new row, and there is no validation. A purpose-built CLI tool is a better approach for a bootstrapping operation that will be run exactly once per environment.

---

## Design Decision: Why Direct DB Access?
<!-- [COPY EDIT] CMOS 6.63 (heading): "Design Decision: Why Direct DB Access?" — the portion after the colon is a complete sentence (an interrogative clause), so initial caps are correct. -->
<!-- [STRUCTURAL] This sidebar is one of the strongest moments in the chapter. The three bullets (API can't do it; ops tool not feature; simplest correct solution) justify the design well. -->

The admin CLI connects directly to PostgreSQL using GORM, bypassing the auth service entirely. This might seem wrong -- elsewhere in this project, we have been careful to route all operations through gRPC. But bootstrapping is different:

<!-- [LINE EDIT] "might seem wrong -- elsewhere in this project, we have been careful to route all operations through gRPC." — good; no change. -->

- **The gRPC API cannot do this.** There is no `PromoteUser` RPC, and adding one would create a security surface area that needs protection (who can call it? the first admin? how do you authorize the first admin?).
<!-- [LINE EDIT] "a security surface area" → "a security surface" (surface area is tautological; the surface *is* the area). -->
<!-- [COPY EDIT] CMOS 6.75: parenthetical series of questions. Acceptable; each mini-question could be a sentence on its own, but the compressed form works for a parenthetical aside. -->
- **This is an ops tool, not a feature.** It will be run once by an operator, not called by other services. It does not need to participate in the service mesh, emit events, or be load-balanced.
<!-- [COPY EDIT] "load-balanced" — compound adjective used predicatively ("need to … be load-balanced"); hyphen still common and acceptable. CMOS 7.89. -->
- **Direct DB access is the simplest correct solution.** The CLI reuses the same GORM model as the auth service, so it stays in sync with schema migrations.

<!-- [STRUCTURAL] The Kubernetes parenthetical is apt but wordy. Consider trimming to a single sentence. -->
<!-- [LINE EDIT] "Kubernetes operators often run one-off jobs (`kubectl exec`, init containers, or `Job` resources) that interact with databases directly." — 21 words, fine. -->
In production environments, this pattern is common. Kubernetes operators often run one-off jobs (`kubectl exec`, init containers, or `Job` resources) that interact with databases directly. The important thing is to keep these tools in the same repository as the service they operate on, so they stay in sync.
<!-- [COPY EDIT] "Kubernetes operators" — ambiguous. In Kubernetes terminology, "Operator" (capital O) refers specifically to the controller pattern (CRD + custom controller). You almost certainly mean "human operators running Kubernetes" here. Recommend: "Kubernetes operators often run one-off jobs…" → "In Kubernetes, teams often run one-off jobs…" Please verify intended meaning. -->

---

## Code Walkthrough

The entire CLI lives in a single file:

<!-- [STRUCTURAL] Good. Single-file scope is called out so the reader knows what to expect. -->

```go
// services/auth/cmd/admin/main.go

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fesoliveira014/library-system/services/auth/internal/model"
)
```

<!-- [COPY EDIT] Please verify: the imports shown here include `gorm.io/driver/postgres` and `gorm.io/gorm`, but the code on line 80 uses `pkgdb.Open(dsn, pkgdb.Config{})` — meaning a `pkg/db` helper is imported but not shown. Either the import block is incomplete, or `pkgdb.Open` is not used and the code on line 80 should call `gorm.Open(postgres.Open(dsn), &gorm.Config{})` directly. Please reconcile. -->

<!-- [LINE EDIT] "The imports tell the story: this is a GORM + bcrypt program that reuses the auth service's `model.User` type. No gRPC, no HTTP, no Kafka." — punchy and good. Keep. -->
The imports tell the story: this is a GORM + bcrypt program that reuses the auth service's `model.User` type. No gRPC, no HTTP, no Kafka.

### Flag Parsing

```go
func main() {
	email := flag.String("email", "", "admin email (required)")
	password := flag.String("password", "", "admin password (required)")
	name := flag.String("name", "", "admin display name (required)")
	flag.Parse()

	if *email == "" || *password == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "Usage: admin --email EMAIL --password PASSWORD --name NAME")
		fmt.Fprintln(os.Stderr, "Requires DATABASE_URL environment variable")
		os.Exit(1)
	}
```

<!-- [LINE EDIT] "Go's `flag` package is intentionally simple." → "Go's `flag` package is intentionally minimal." Reason: "simple" was cut as a filler per style guide; "minimal" conveys the design intent more precisely. -->
<!-- [LINE EDIT] "Flags are defined with `flag.String` (or `flag.Int`, `flag.Bool`, etc.), which returns a pointer. After `flag.Parse()`, the pointer is dereferenced to get the value. This is one of Go's more awkward APIs -- the pointer indirection exists because `flag.Parse` needs to write values after the variables are declared." — 49 words; borderline long but clear. Keep. -->
Go's `flag` package is intentionally simple. Flags are defined with `flag.String` (or `flag.Int`, `flag.Bool`, etc.), which returns a pointer. After `flag.Parse()`, the pointer is dereferenced to get the value. This is one of Go's more awkward APIs -- the pointer indirection exists because `flag.Parse` needs to write values after the variables are declared.
<!-- [COPY EDIT] CMOS 6.43: "e.g.," and "etc." — "etc." follows a list where "e.g." was not used, so no redundancy. Comma after "etc." is correct in mid-sentence. -->

<!-- [LINE EDIT] "If you are coming from Java/Kotlin, this is the equivalent of a bare-bones `args` parser." → "If you are coming from Java/Kotlin, this is roughly equivalent to a bare-bones `args` parser." Reason: "the equivalent of" is over-strong — Go's `flag` does more than a bare `args` parser (it handles `--flag=value`, help text, and default values). -->
If you are coming from Java/Kotlin, this is the equivalent of a bare-bones `args` parser. For more complex CLIs, libraries like `cobra` or `urfave/cli` provide subcommands and help generation, but for a three-flag tool, `flag` is the right choice.
<!-- [COPY EDIT] "Cobra" — CMOS 8.154 (product capitalization). The library is named "Cobra" (capital C) in its README and project branding. Same applies on line 182. Recommend "`cobra`" → "Cobra" in prose, keep backticks only when referring to the import path or binary. -->
<!-- [COPY EDIT] "urfave/cli" — canonical spelling is the GitHub path `urfave/cli`. Acceptable as-is in code voice. -->
<!-- [COPY EDIT] "three-flag tool" — CMOS 7.89 compound adjective before noun; hyphen correct. -->

### Database Connection

```go
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := pkgdb.Open(dsn, pkgdb.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
```

<!-- [COPY EDIT] Please verify: `pkgdb.Open` signature and the `pkgdb.Config{}` zero value. If this is the project's `pkg/db` helper, the claim "sets the same connection-pool defaults the service uses" depends on `Config{}` triggering those defaults — confirm the helper handles the zero value this way rather than requiring explicit pool settings. -->
The `DATABASE_URL` is a standard PostgreSQL connection string (e.g., `postgres://user:pass@host:port/dbname?sslmode=disable`). The `pkgdb.Open` helper sets the same connection-pool defaults the service uses (see [Chapter 2](../ch02/repository-pattern.md#configuring-the-connection-pool)).
<!-- [COPY EDIT] "PostgreSQL" — product capitalization correct (CMOS 8.154). -->
<!-- [COPY EDIT] CMOS 6.43: "e.g., " — comma after "e.g." correct. -->
<!-- [FINAL] Please verify cross-reference: `../ch02/repository-pattern.md#configuring-the-connection-pool` — confirm the heading slug resolves. Mkdocs lowercases and hyphenates; a heading like "Configuring the Connection Pool" should produce `#configuring-the-connection-pool`. Looks fine. -->

<!-- [STRUCTURAL] The "Why no AutoMigrate?" blockquote is the best didactic moment in the section — explains a real engineering hazard (schema drift) with concrete reasoning. Keep. -->
> **Why no `AutoMigrate`?** An early draft of this CLI called `db.AutoMigrate(&model.User{})` on startup so the tool would work against a fresh database. That turned out to be the wrong instinct — the `auth` service already runs versioned `golang-migrate` migrations on startup, so by the time anyone runs this CLI the `users` table definitely exists. Calling `AutoMigrate` on top of a database that was provisioned with raw SQL migrations is a recipe for schema drift: GORM happily adds columns it sees in the struct, but never drops ones it doesn't, and it writes nothing to `schema_migrations`. The CLI just assumes the schema is in place.
<!-- [LINE EDIT] "The CLI just assumes the schema is in place." → "The CLI assumes the schema is in place." Reason: "just" is a style-guide filler word. -->
<!-- [COPY EDIT] "golang-migrate" — the library is literally named `golang-migrate/migrate` on GitHub; prose spelling "golang-migrate" is canonical. Backticks optional. -->
<!-- [COPY EDIT] "GORM happily adds columns it sees in the struct, but never drops ones it doesn't" — CMOS 6.19: the sentence is two clauses joined by "but"; independent clauses take a comma before "but", which you have. Correct. -->
<!-- [FINAL] "doesn't" — possessive/contraction correct (GORM "doesn't"). -->

### Password Hashing

```go
	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}
	hashStr := string(hash)
```

<!-- [COPY EDIT] Please verify: `bcrypt.DefaultCost` equals 10 in `golang.org/x/crypto/bcrypt`. Confirmed in package constant; safe to state. -->
This uses the same `bcrypt.DefaultCost` (10 rounds) as the auth service's registration flow. The password is never stored in plaintext. The `hashStr` variable exists because `model.User.PasswordHash` is a `*string` (nullable, since OAuth users do not have passwords).
<!-- [COPY EDIT] "OAuth" — CMOS 8.154; industry-standard caps. Correct. -->
<!-- [COPY EDIT] "plaintext" — closed compound, widely accepted. M-W lists both "plain text" and "plaintext". Consistency check across chapter: this is the only use. Fine. -->

### Idempotent Upsert

<!-- [STRUCTURAL] Heading is accurate. "Upsert" is the right term; you also use the behaviour description ("logic is intentionally idempotent") below. -->

```go
	var existing model.User
	result := db.Where("email = ?", *email).First(&existing)
	if result.Error == nil {
		existing.Role = "admin"
		existing.PasswordHash = &hashStr
		existing.Name = *name
		if err := db.Save(&existing).Error; err != nil {
			log.Fatalf("failed to update user: %v", err)
		}
		fmt.Printf("Updated existing user %s to admin role\n", *email)
		return
	}

	user := model.User{
		Email:        *email,
		PasswordHash: &hashStr,
		Name:         *name,
		Role:         "admin",
	}
	if err := db.Create(&user).Error; err != nil {
		log.Fatalf("failed to create admin user: %v", err)
	}
	fmt.Printf("Created admin user: %s (%s)\n", *email, user.ID)
}
```

<!-- [STRUCTURAL] One concern: the lookup branch treats *any* non-nil `result.Error` as "record not found" and proceeds to create a new user. If the DB is down or the query has a syntax issue, the create will also fail, but it will fail less informatively than a direct check against `gorm.ErrRecordNotFound`. Worth a one-line query: should the narrative acknowledge this? -->
<!-- [COPY EDIT] Please verify: the prose on line 138 below states "`First` returns an error if no record is found (GORM's `ErrRecordNotFound`)". Consider whether the code should explicitly branch on `errors.Is(result.Error, gorm.ErrRecordNotFound)` for robustness; if the author deliberately kept the looser check, add a line acknowledging the simplification. -->

The logic is intentionally idempotent:

1. If a user with that email already exists, update their role to `"admin"` and reset their password and name.
2. If no user exists, create a new one with the `"admin"` role.

<!-- [LINE EDIT] "This means you can safely run the CLI multiple times. If you forget your admin password, just re-run it with a new one." — "just" is a filler. Drop: "If you forget your admin password, re-run it with a new one." -->
This means you can safely run the CLI multiple times. If you forget your admin password, just re-run it with a new one. If a regular user needs to be promoted, point the CLI at their email.

<!-- [LINE EDIT] "Note the use of GORM's `First` and `Save` -- `First` returns an error if no record is found (GORM's `ErrRecordNotFound`), and `Save` performs a full update on the existing record. This is different from `Updates`, which only updates non-zero fields." — 41 words; still readable; keep. -->
Note the use of GORM's `First` and `Save` -- `First` returns an error if no record is found (GORM's `ErrRecordNotFound`), and `Save` performs a full update on the existing record. This is different from `Updates`, which only updates non-zero fields.
<!-- [COPY EDIT] em dash `Save` -- `First` → `Save` — `First` (CMOS 6.85). -->

---

## Usage

<!-- [LINE EDIT] "With the stack running via Docker Compose, the auth database is exposed on port 5434 (as defined in `deploy/.env`):" → "With the stack running via Docker Compose, the auth database is exposed on port 5434 (set in `deploy/.env`):" Reason: "as defined in" is slightly formal for the register. -->
With the stack running via Docker Compose, the auth database is exposed on port 5434 (as defined in `deploy/.env`):

```bash
DATABASE_URL="postgres://postgres:postgres@localhost:5434/auth?sslmode=disable" \
  go run ./services/auth/cmd/admin \
    --email admin@library.local \
    --password admin123 \
    --name "Library Admin"
```

Expected output:

```
Created admin user: admin@library.local (a1b2c3d4-...)
```

If you run it again with the same email:

```
Updated existing user admin@library.local to admin role
```

### Verifying the Account

You can verify the admin account was created correctly by logging in through the gateway UI at `http://localhost:8080/login`, or by querying the database directly:

```bash
psql "postgres://postgres:postgres@localhost:5434/auth?sslmode=disable" \
  -c "SELECT email, role FROM users WHERE email = 'admin@library.local';"
```

---

## Key Takeaways

<!-- [STRUCTURAL] Takeaways mirror the design-decision bullets. Good reinforcement. -->

- **Bootstrapping tools bypass the API by design.** They solve problems that the API cannot solve (creating the first privileged account).
- **Reuse domain models.** The CLI imports `model.User` from the auth service, keeping it in sync with the schema automatically.
- **Idempotency matters.** Running the tool twice should not fail or create duplicates.
- **Keep it simple.** A single-file `main.go` with `flag` is appropriate for a tool with three arguments. Reach for `cobra` when you have subcommands.
<!-- [COPY EDIT] "Cobra" capitalization (CMOS 8.154) — same note as earlier; "cobra" → "Cobra" in prose. -->
<!-- [LINE EDIT] "Keep it simple." — "simple" is a style-guide filler, but here it is the *label* of the takeaway, not noise. Leave. -->
