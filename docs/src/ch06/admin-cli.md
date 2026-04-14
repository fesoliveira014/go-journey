# 6.1 Admin CLI

## The Problem

The auth service's `Register` RPC always creates users with the `"user"` role. This is the correct default -- you do not want new sign-ups to be admins. But it means there is no way to create the first admin account through the application itself. You could run raw SQL:

```sql
UPDATE users SET role = 'admin' WHERE email = 'admin@example.com';
```

That works, but it is fragile. You need to know the table and column names, you need to remember to hash the password if you are inserting a new row, and there is no validation. A purpose-built CLI tool is a better approach for a bootstrapping operation that will be run exactly once per environment.

---

## Design Decision: Why Direct DB Access?

The admin CLI connects directly to PostgreSQL using GORM, bypassing the auth service entirely. This might seem wrong -- elsewhere in this project, we have been careful to route all operations through gRPC. But bootstrapping is different:

- **The gRPC API cannot do this.** There is no `PromoteUser` RPC, and adding one would create a security surface area that needs protection (who can call it? the first admin? how do you authorize the first admin?).
- **This is an ops tool, not a feature.** It will be run once by an operator, not called by other services. It does not need to participate in the service mesh, emit events, or be load-balanced.
- **Direct DB access is the simplest correct solution.** The CLI reuses the same GORM model as the auth service, so it stays in sync with schema migrations.

In production environments, this pattern is common. Kubernetes operators often run one-off jobs (`kubectl exec`, init containers, or `Job` resources) that interact with databases directly. The important thing is to keep these tools in the same repository as the service they operate on, so they stay in sync.

---

## Code Walkthrough

The entire CLI lives in a single file:

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

Go's `flag` package is intentionally simple. Flags are defined with `flag.String` (or `flag.Int`, `flag.Bool`, etc.), which returns a pointer. After `flag.Parse()`, the pointer is dereferenced to get the value. This is one of Go's more awkward APIs -- the pointer indirection exists because `flag.Parse` needs to write values after the variables are declared.

If you are coming from Java/Kotlin, this is the equivalent of a bare-bones `args` parser. For more complex CLIs, libraries like `cobra` or `urfave/cli` provide subcommands and help generation, but for a three-flag tool, `flag` is the right choice.

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

The `DATABASE_URL` is a standard PostgreSQL connection string (e.g., `postgres://user:pass@host:port/dbname?sslmode=disable`). The `pkgdb.Open` helper sets the same connection-pool defaults the service uses (see [Chapter 2](../ch02/repository-pattern.md#configuring-the-connection-pool)).

> **Why no `AutoMigrate`?** An early draft of this CLI called `db.AutoMigrate(&model.User{})` on startup so the tool would work against a fresh database. That turned out to be the wrong instinct — the `auth` service already runs versioned `golang-migrate` migrations on startup, so by the time anyone runs this CLI the `users` table definitely exists. Calling `AutoMigrate` on top of a database that was provisioned with raw SQL migrations is a recipe for schema drift: GORM happily adds columns it sees in the struct, but never drops ones it doesn't, and it writes nothing to `schema_migrations`. The CLI just assumes the schema is in place.

### Password Hashing

```go
	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}
	hashStr := string(hash)
```

This uses the same `bcrypt.DefaultCost` (10 rounds) as the auth service's registration flow. The password is never stored in plaintext. The `hashStr` variable exists because `model.User.PasswordHash` is a `*string` (nullable, since OAuth users do not have passwords).

### Idempotent Upsert

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

The logic is intentionally idempotent:

1. If a user with that email already exists, update their role to `"admin"` and reset their password and name.
2. If no user exists, create a new one with the `"admin"` role.

This means you can safely run the CLI multiple times. If you forget your admin password, just re-run it with a new one. If a regular user needs to be promoted, point the CLI at their email.

Note the use of GORM's `First` and `Save` -- `First` returns an error if no record is found (GORM's `ErrRecordNotFound`), and `Save` performs a full update on the existing record. This is different from `Updates`, which only updates non-zero fields.

---

## Usage

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

- **Bootstrapping tools bypass the API by design.** They solve problems that the API cannot solve (creating the first privileged account).
- **Reuse domain models.** The CLI imports `model.User` from the auth service, keeping it in sync with the schema automatically.
- **Idempotency matters.** Running the tool twice should not fail or create duplicates.
- **Keep it simple.** A single-file `main.go` with `flag` is appropriate for a tool with three arguments. Reach for `cobra` when you have subcommands.
