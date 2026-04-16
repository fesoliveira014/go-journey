## Overall Assessment

The architecture is sound. The layered service structure (handler → service → repository), the use of interfaces at layer boundaries, Go workspace monorepo layout, database-per-service, and the BFF gateway pattern all follow established microservices and Go conventions. The progression from local Docker Compose to Kubernetes to Terraform-managed AWS is well-sequenced. What follows are areas where the project either teaches patterns that conflict with established best practices or where the code contains latent issues the reader may internalize.

---

## 1. PostgreSQL Error Detection via String Matching

**Location:** `services/catalog/internal/repository/book.go`, `services/auth/internal/repository/user.go`

```go
func isDuplicateKeyError(err error) bool {
    msg := err.Error()
    return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "SQLSTATE 23505")
}
```

This is fragile. The `pgx` driver (which GORM uses under the hood via `gorm.io/driver/postgres`) returns structured `*pgconn.PgError` values with a typed `Code` field. The correct Go approach is:

```go
import "github.com/jackc/pgx/v5/pgconn"

var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" {
    return true
}
```

String matching on error messages is explicitly warned against in the Go blog's ["Error handling and Go"](https://go.dev/blog/error-handling-and-go) and in Dave Cheney's ["Don't just check errors, handle them gracefully"](https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully). Error messages are not a stable API — driver version upgrades, locale changes, or switching from `pgx` to `lib/pq` can silently break this detection. Since this pattern is repeated across two services and taught as "the way" in Chapter 2, readers are likely to carry it forward.

---

## 2. Check-Then-Act Race in Reservation Creation

**Location:** Reservation service (`services/reservation/internal/service/`)

The reservation flow is: (1) call the Catalog service via gRPC to check `available_copies > 0`, then (2) insert the reservation in a separate database. This is a textbook TOCTOU (time-of-check to time-of-use) race condition. Two users reserving the last copy simultaneously will both pass the availability check, both create reservations, and the Kafka events will decrement `available_copies` to -1 (or 0 if the SQL guard catches one).

The chapter acknowledges this in Exercise 4 of section 7.2 but presents the code as the working implementation without any mitigation. For a tutorial targeting engineers who will build production systems, this warrants at least an inline comment in the code or a "Known Limitations" callout in the body text — not just an exercise. Sam Newman's *Building Microservices* (2nd ed., Chapter 6) discusses this pattern extensively under "Sagas" and recommends either a reservation-with-timeout at the source of truth or an optimistic concurrency check.

The SQL guard in `UpdateAvailability` (`WHERE available_copies + ? >= 0`) prevents negative counts but does not prevent overbooking — the reservation already exists in the reservation database.

---

## 3. Sarama Is in Maintenance Mode

**Location:** Chapter 7, all Kafka-related code

The project uses `github.com/IBM/sarama` and presents it as "the most established pure-Go implementation." While historically true, Sarama has been in effective maintenance mode since IBM took over stewardship. The [Sarama README itself](https://github.com/IBM/sarama) notes that active development has slowed, and the community has largely moved toward `github.com/twmb/franz-go`, which has a more idiomatic Go API, better performance, and active development.

The chapter does mention `franz-go` as "a newer pure-Go client with a more modern API," which is fair. However, for a tutorial written in 2026 teaching people how to build new systems, defaulting to the library the community is moving away from creates maintenance debt for anyone who follows the tutorial as a starting point. The [franz-go comparison document](https://github.com/twmb/franz-go/blob/master/docs/comparisons.md) details the differences.

---

## 4. Hardcoded Default Secrets as Fallbacks

**Location:** `services/auth/cmd/main.go`, `services/reservation/cmd/main.go`

```go
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    jwtSecret = "dev-secret-change-in-production"
}
```

This pattern means the service *silently starts with a known, weak secret* if the environment variable is missing. In production, a misconfigured deployment (forgotten env var, typo in ConfigMap key) would result in a running service that accepts tokens signed with a publicly known string. The service should refuse to start without a JWT secret. The 12-Factor App methodology ([Factor III: Config](https://12factor.net/config)) and the Go standard library's own practices suggest failing fast on missing required config.

Chapter 14 introduces External Secrets Operator and proper secret management, but by then the reader has been using this pattern for 13 chapters. A better teaching pattern:

```go
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    log.Fatal("JWT_SECRET environment variable is required")
}
```

with a `.env` file or Docker Compose env block providing the dev default externally.

---

## 5. Mixed Migration Strategies

**Location:** `services/auth/cmd/admin/main.go` vs. all service `main.go` files

The services use `golang-migrate` with versioned SQL files (correctly). But the admin CLI uses `db.AutoMigrate(&model.User{})`:

```go
if err := db.AutoMigrate(&model.User{}); err != nil {
    log.Fatalf("failed to migrate: %v", err)
}
```

Chapter 2 explicitly warns that "AutoMigrate is useful in development but dangerous in production: it never drops columns and tracks no history." Using it in the admin CLI contradicts this advice. If the CLI is run against a database that has already been migrated with `golang-migrate`, GORM's AutoMigrate may produce schema drift — it could add columns that the migration system doesn't know about, creating an inconsistency in `schema_migrations`. The CLI should use the same `runMigrations()` pattern as the services, or simply assume the schema is already in place (which it is, since the auth service runs migrations on startup).

---

## 6. No Database Connection Pool Configuration

**Location:** All service `main.go` files

Every service opens a GORM connection with:

```go
db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
```

No connection pool settings are configured — no `SetMaxOpenConns`, `SetMaxIdleConns`, or `SetConnMaxLifetime`. GORM/pgx defaults to unlimited open connections. Under load (or during Kubernetes pod restarts with multiple replicas), this can exhaust PostgreSQL's `max_connections` (default 100). The [GORM documentation](https://gorm.io/docs/connecting_to_the_database.html#Connection-Pool) explicitly recommends configuring the pool:

```go
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(25)
sqlDB.SetMaxIdleConns(5)
sqlDB.SetConnMaxLifetime(5 * time.Minute)
```

For a tutorial that deploys to Kubernetes with multiple replicas and managed RDS instances, this is worth teaching.

---

## 7. Flash Messages in Unencrypted Cookies

**Location:** `services/gateway/internal/handler/render.go`

```go
func setFlash(w http.ResponseWriter, message string) {
    http.SetCookie(w, &http.Cookie{
        Name:     "flash",
        Value:    message,
        Path:     "/",
        MaxAge:   10,
        HttpOnly: true,
    })
}
```

The flash message value is stored as plaintext in the cookie. A user (or intermediary) can modify the cookie value and inject arbitrary text into the page. Since the value is rendered in the HTML template, this is a stored XSS vector unless `html/template` escapes it (which Go's template engine does by default). The chapter should note that this works only because Go's `html/template` auto-escapes, and that in frameworks without auto-escaping (or if someone switches to `text/template`), this would be a vulnerability. Alternatively, use a signed cookie or server-side session. The Gorilla securecookie package provides HMAC-signed cookies as a well-established solution.

---

## 8. No `t.Parallel()` in Test Suites

**Location:** All `_test.go` files across the project

None of the unit tests call `t.Parallel()`. For tests using in-memory mocks (like the service layer tests), parallel execution is safe, free, and exposes shared-state bugs. The Go testing documentation and Mitchell Hashimoto's ["Advanced Testing with Go"](https://about.sourcegraph.com/go/advanced-testing-in-go) talk recommend defaulting to `t.Parallel()` for unit tests. For a project that emphasizes testing idioms, this is a missed teaching opportunity.

---

## 9. Reservation Expiration-on-Read Pattern

**Location:** Chapter 7.2, `services/reservation/internal/service/`

The reservation service expires overdue reservations lazily — only when someone reads them. This means a book that was reserved and never returned stays "unavailable" in the catalog indefinitely until someone happens to query that user's reservations. The Kafka consumer that updates `available_copies` only fires when a `reservation.expired` event is published, which only happens during a read.

The chapter presents this as a conscious choice, which is fair. But it doesn't adequately address the consequence: the catalog's `available_copies` can become permanently stale if no one triggers the read path. A background goroutine running a periodic `UPDATE reservations SET status = 'expired' WHERE status = 'active' AND due_at < now()` with corresponding Kafka events is the standard approach, and the chapter mentions it only in an exercise. For a tutorial that teaches event-driven architecture, this gap between the read path and the event path is worth resolving in the main text.

---

## 10. HTMX Loaded from Unpkg CDN

**Location:** `services/gateway/templates/base.html`

```html
<script src="https://unpkg.com/htmx.org@2.0.4"></script>
```

Loading JavaScript from a third-party CDN without a Subresource Integrity (SRI) hash means a CDN compromise can inject malicious code into every page. The [HTMX documentation](https://htmx.org/docs/#installing) provides SRI hashes for every release. The fix is straightforward:

```html
<script src="https://unpkg.com/htmx.org@2.0.4"
        integrity="sha384-..." crossorigin="anonymous"></script>
```

For a project that teaches production hardening in Chapter 14, this is a low-hanging security improvement that belongs in the Chapter 5 code.

---

## Items That Are Sound

The following aspects are handled correctly without need for special praise: the layered architecture with interfaces at boundaries; the use of `internal/` for visibility enforcement; Go workspace monorepo with independent `go.mod` per service; protobuf definitions in a shared `proto/` directory with `buf` for code generation; the gRPC health checking protocol for Kubernetes probes; graceful shutdown with `signal.NotifyContext`; the OTel shared initialization package in `pkg/otel`; structured logging with `slog` and trace ID correlation; the Kustomize base/overlay pattern for environment separation; the Kubernetes security context configuration (`runAsNonRoot`, `readOnlyRootFilesystem`, `drop: ALL`); the CI/CD pipeline structure with Earthly and GitHub Actions; and the use of embedded SQL migrations with `golang-migrate`.