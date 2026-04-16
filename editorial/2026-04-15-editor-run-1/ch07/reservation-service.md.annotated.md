# 7.2 Reservation Service

<!-- [STRUCTURAL] Section scope: domain model → repository → service → handler → wiring, with a focused detour into the TOCTOU trap and expiration. Clean and predictable, matches the layered-architecture teaching strategy. Keep. -->

The reservation service is the second microservice we build from scratch. If you followed Chapter 2 (the catalog service), this one will feel familiar -- same layered architecture, same patterns. That repetition is deliberate. The goal is to show that the patterns are general, not specific to any one domain. Once you internalize the model/repository/service/handler stack, you can stand up a new service quickly.

<!-- [LINE EDIT] "you can stand up a new service quickly" → "you can stand up a new service fast" — slight informal cadence match to the earlier "stand up" verb. Optional. -->

The interesting differences are in the domain logic: state machines, cross-service reads, event publishing, and a two-pronged expiration strategy — lazy expire-on-read for responsiveness plus a background reaper for timeliness.

<!-- [COPY EDIT] Em dash style here (no spaces) differs from the spaced en dash ` -- ` used elsewhere in this file and in 7.1. Normalize to one chapter-wide. (CMOS 6.85) -->
<!-- [COPY EDIT] "cross-service reads" — this file later establishes that the sync call to catalog is a guarded *decrement*, not just a read. "Cross-service reads" in this intro oversells the read framing (same issue flagged in 7.1). Consider "cross-service coordination" or "cross-service calls". -->
<!-- [COPY EDIT] "two-pronged" — hyphenated compound adjective before noun (CMOS 7.81). Correct. -->

---

## Domain Model

The reservation domain is small. A `Reservation` tracks who reserved which book, when, and what happened to it:

```go
// services/reservation/internal/model/model.go

type Reservation struct {
    ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
    UserID     uuid.UUID  `gorm:"type:uuid;not null"`
    BookID     uuid.UUID  `gorm:"type:uuid;not null"`
    Status     string     `gorm:"type:varchar(20);not null;default:'active'"`
    ReservedAt time.Time  `gorm:"type:timestamptz;not null;default:now()"`
    DueAt      time.Time  `gorm:"type:timestamptz;not null"`
    ReturnedAt *time.Time `gorm:"type:timestamptz"`
}

const (
    StatusActive   = "active"
    StatusReturned = "returned"
    StatusExpired  = "expired"
)
```

A few things to note:

**State machine.** The `Status` field has three valid values, and the transitions are strict: `active -> returned` (user returned the book) and `active -> expired` (the due date passed). There is no transition from `returned` to `active` or from `expired` to `returned`. The service enforces these rules in the service layer, not in the database -- though you could add a CHECK constraint for defense in depth.

<!-- [COPY EDIT] `active -> returned` uses ASCII arrow in prose; some style guides prefer the Unicode arrow `→` in running text (while keeping ASCII in code blocks). The book uses `→` elsewhere (e.g., the index architecture diagram). Standardize. -->
<!-- [COPY EDIT] "defense in depth" — established technical idiom, no hyphen needed as a noun phrase; if used attributively ("defense-in-depth strategy") it would be hyphenated. Correct as written. -->

**`ReturnedAt` is a pointer.** A `*time.Time` is Go's way of expressing a nullable timestamp. When the reservation is active, `ReturnedAt` is `nil`. When returned, it is set. GORM understands pointer fields as nullable columns. In Kotlin, you would write `val returnedAt: Instant?` -- same concept, different syntax.

<!-- [STRUCTURAL] Nice Kotlin analogy for the target reader. Keep. -->

**Sentinel errors.** The model package defines domain-specific errors:

```go
var (
    ErrReservationNotFound = errors.New("reservation not found")
    ErrAlreadyReturned     = errors.New("reservation already returned or expired")
    ErrMaxReservations     = errors.New("maximum active reservations reached")
    ErrNoAvailableCopies   = errors.New("no available copies")
    ErrPermissionDenied    = errors.New("permission denied")
)
```

These are package-level variables (not types) used with `errors.Is()`. This is the same pattern we used in the catalog service. The handler layer maps these to gRPC status codes -- the domain errors stay clean of transport concerns.

<!-- [LINE EDIT] "These are package-level variables (not types) used with errors.Is()." → "These are package-level variables — not types — matched with errors.Is()." — Slightly more fluid; "matched" is more precise than "used". Optional. -->

---

## Repository Layer

The repository is thin. It wraps GORM queries and translates `gorm.ErrRecordNotFound` into the domain's `ErrReservationNotFound`:

```go
// services/reservation/internal/repository/repository.go

type ReservationRepository struct {
    db *gorm.DB
}

func NewReservationRepository(db *gorm.DB) *ReservationRepository {
    return &ReservationRepository{db: db}
}

func (r *ReservationRepository) Create(ctx context.Context, res *model.Reservation) (*model.Reservation, error) {
    if err := r.db.WithContext(ctx).Create(res).Error; err != nil {
        return nil, err
    }
    return res, nil
}

func (r *ReservationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Reservation, error) {
    var res model.Reservation
    if err := r.db.WithContext(ctx).First(&res, "id = ?", id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, model.ErrReservationNotFound
        }
        return nil, err
    }
    return &res, nil
}
```

Notice `CountActive` -- it counts the user's currently active reservations. This powers the "max reservations" business rule:

<!-- [LINE EDIT] "it counts the user's currently active reservations" → "it counts the user's active reservations" — "currently" is redundant with "active". -->

```go
func (r *ReservationRepository) CountActive(ctx context.Context, userID uuid.UUID) (int64, error) {
    var count int64
    err := r.db.WithContext(ctx).Model(&model.Reservation{}).
        Where("user_id = ? AND status = ?", userID, model.StatusActive).
        Count(&count).Error
    return count, err
}
```

Every method takes a `context.Context` and passes it to GORM via `WithContext(ctx)`. This ensures that request-scoped deadlines, cancellations, and tracing spans propagate through to the database driver. If you forget `WithContext`, the query still works but loses its connection to the calling context -- timeouts will not cancel it, and traces will not include it.

<!-- [STRUCTURAL] Good callout — easy to get wrong, worth flagging. Keep. -->

The `ListByUser` method returns all reservations for a user, ordered by most recent first:

```go
func (r *ReservationRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*model.Reservation, error) {
    var reservations []*model.Reservation
    err := r.db.WithContext(ctx).
        Where("user_id = ?", userID).
        Order("reserved_at DESC").
        Find(&reservations).Error
    return reservations, err
}
```

Compared to the catalog repository, this one is simpler -- no pagination, no filtering, no full-text search. A real system would add those, but for learning purposes, this is enough to demonstrate the pattern.

<!-- [LINE EDIT] "for learning purposes, this is enough to demonstrate the pattern" → "for learning purposes, this is enough." — The "to demonstrate the pattern" tail is filler. -->

---

## Service Layer

The service layer is where the interesting logic lives. Let us look at its dependencies:

<!-- [LINE EDIT] "The service layer is where the interesting logic lives." → "The service layer is where the domain logic lives." — "Interesting" is subjective filler; "domain" is precise. -->
<!-- [LINE EDIT] "Let us look at its dependencies" → "Its dependencies:" — Direct, avoids the "let us look" preamble. -->

```go
// services/reservation/internal/service/service.go

type ReservationService struct {
    repo      ReservationRepository
    catalog   catalogv1.CatalogServiceClient
    publisher EventPublisher
    maxActive int
}
```

Three things to call out:

1. **`repo` is an interface**, not a concrete type. The service defines the interface it needs (`ReservationRepository`), and the repository satisfies it. This is Go's implicit interface satisfaction -- the repository never declares `implements ReservationRepository`. If it has the right methods, it fits.

2. **`catalog` is a gRPC client.** The reservation service calls the catalog service synchronously to check book availability before creating a reservation. This is a cross-service read -- the reservation service does not own the book data, so it asks the service that does.

<!-- [STRUCTURAL] Same issue as 7.1: "cross-service read" / "check book availability" is the wrong framing. The implementation calls `UpdateAvailability(-1)` — a guarded write — which serves *as* the availability check. This description contradicts the TOCTOU section below. Rephrase bullet 2: "**`catalog` is a gRPC client.** Before creating the reservation row, the service calls `catalog.UpdateAvailability(bookID, -1)` — a guarded decrement that doubles as the availability check. See *The TOCTOU trap* below for why." -->

3. **`publisher` is an interface.** The `EventPublisher` interface has one method: `Publish(ctx, event) error`. The Kafka publisher implements it, but in tests you can substitute a mock. This is the same dependency inversion pattern used throughout the codebase.

### Creating a Reservation

The `CreateReservation` method enforces all the business rules. The interesting part is the order of operations — we decrement availability **before** creating the reservation row, not after. The next section (_The TOCTOU trap_) explains why.

<!-- [STRUCTURAL] Good pre-pointer to the TOCTOU section. Tighten: "The ordering is the interesting bit — we decrement availability **before** inserting the reservation row. The next section explains why." -->
<!-- [COPY EDIT] "(_The TOCTOU trap_)" — inline italic cross-reference inside parentheses reads a bit cluttered. Consider: "The next section, _The TOCTOU trap_, explains why." -->
<!-- [COPY EDIT] Em dash inside this paragraph vs. spaced en dash elsewhere — normalize chapter-wide. -->

```go
func (s *ReservationService) CreateReservation(ctx context.Context, bookID uuid.UUID) (*model.Reservation, error) {
    userID, err := pkgauth.UserIDFromContext(ctx)
    if err != nil {
        return nil, fmt.Errorf("user not authenticated: %w", err)
    }

    // Rule 1: enforce max active reservations per user
    count, err := s.repo.CountActive(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("count active reservations: %w", err)
    }
    if count >= int64(s.maxActive) {
        return nil, model.ErrMaxReservations
    }

    // Rule 2: ask catalog to atomically decrement available_copies. Catalog's
    // guarded UPDATE (WHERE available_copies + ? >= 0) is the single source
    // of truth for "is a copy available?" — no in-service pre-check.
    if _, err := s.catalog.UpdateAvailability(ctx, &catalogv1.UpdateAvailabilityRequest{
        Id:    bookID.String(),
        Delta: -1,
    }); err != nil {
        if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
            return nil, model.ErrNoAvailableCopies
        }
        return nil, fmt.Errorf("reserve availability: %w", err)
    }

    // Create the reservation row. If this fails we must put the copy back
    // or catalog's counter drifts permanently below the real availability.
    now := time.Now()
    res := &model.Reservation{
        UserID: userID, BookID: bookID, Status: model.StatusActive,
        ReservedAt: now, DueAt: now.Add(loanDuration), // 14 days
    }
    created, err := s.repo.Create(ctx, res)
    if err != nil {
        if _, rollbackErr := s.catalog.UpdateAvailability(ctx, &catalogv1.UpdateAvailabilityRequest{
            Id: bookID.String(), Delta: 1,
        }); rollbackErr != nil {
            slog.ErrorContext(ctx, "failed to compensate availability", ...)
        }
        return nil, fmt.Errorf("create reservation: %w", err)
    }

    // Publish event (fire and log on failure)
    if err := s.publisher.Publish(ctx, ReservationEvent{ ... }); err != nil {
        slog.ErrorContext(ctx, "failed to publish event", ...)
    }

    return created, nil
}
```

<!-- [STRUCTURAL] The `ReservationEvent{ ... }` in the publish call elides the fields. Since the producer struct is shown in 7.1, a reader tracing the flow will notice the gap. Suggest either (a) show the full event payload inline (small), or (b) add an inline comment `// ReservationEvent{Type: "reservation.created", ReservationID: created.ID.String(), UserID: userID.String(), BookID: bookID.String(), Timestamp: now}`. Readers should not have to jump back to 7.1 to see what gets published. -->

The user ID comes from the context via `pkgauth.UserIDFromContext`. The auth middleware (a gRPC interceptor in this case) validates the JWT token and injects the user ID into the context before the handler runs. This is the same pattern the gateway uses -- extract auth info from the context, not from function parameters.

<!-- [COPY EDIT] "JWT token" — "token" is redundant; JWT = JSON Web Token. "JWT" alone is sufficient, but "JWT token" is widely used idiomatically. Not worth enforcing. -->

The loan duration is a package-level constant: `const loanDuration = 14 * 24 * time.Hour`. This is idiomatic Go -- constants for configuration that does not change at runtime. If this needed to be configurable per environment, it would move to a constructor parameter (like `maxActive`).

### The TOCTOU trap (and why we decrement first)

<!-- [COPY EDIT] Heading: "The TOCTOU trap (and why we decrement first)" — mixed case (TOCTOU all caps, "trap" lowercase). Consider title-casing: "The TOCTOU Trap (and Why We Decrement First)" to match sibling H3s. Alternatively, use sentence case chapter-wide. Pick one. -->

An earlier version of `CreateReservation` looked natural:

```go
book, _ := s.catalog.GetBook(ctx, ...)
if book.AvailableCopies <= 0 {
    return nil, model.ErrNoAvailableCopies
}
// ...create reservation, then later somebody decrements availability...
```

It is also **wrong** under concurrency. This is a textbook [Time-Of-Check-to-Time-Of-Use][toctou] bug: two requests for the last copy of a book call `GetBook` in parallel, both see `AvailableCopies == 1`, both pass the guard, both create reservations. The catalog ends up with `available_copies = -1` or, worse, two users hold the same physical copy.

<!-- [COPY EDIT] Capitalization of "Time-Of-Check-to-Time-Of-Use" — Wikipedia and most security references use "time-of-check to time-of-use" (all lowercase with hyphens, space around "to"). Suggest: "Time-of-check-to-time-of-use". (CMOS 8.161 / domain convention.) -->
<!-- [LINE EDIT] "It is also wrong under concurrency." → "It is also wrong under concurrency — this is a textbook time-of-check-to-time-of-use (TOCTOU) bug." Combines two short sentences, expands the acronym on first use. -->

The fix flips the flow so that the **database** is the gate, not the service:

1. `catalog.UpdateAvailability(book, -1)` runs a guarded `UPDATE` that refuses to go below zero (`WHERE available_copies + ? >= 0`). PostgreSQL's row-level locking during `UPDATE` serialises the two racing decrements — one wins, the other gets zero rows affected and returns `FailedPrecondition`.
2. Only after the decrement succeeds do we create the reservation row.
3. If the reservation insert then fails (DB down, constraint violation, context cancelled), we compensate with `UpdateAvailability(+1)` so catalog's counter does not drift. The compensation is best-effort — if it also fails, the expiration reaper (see _Expiring reservations_) provides a backstop, since an unpaired decrement will eventually be reconciled when other reservations expire and the numbers converge.

<!-- [COPY EDIT] "serialises" → US spelling is "serializes" (CMOS 7.4, though the book's house style may allow UK spellings). Check the rest of the book for consistency; if US English is the house style, change. -->
<!-- [STRUCTURAL] Step 3 claims "an unpaired decrement will eventually be reconciled when other reservations expire and the numbers converge." This is incorrect: if a decrement has no matching reservation row, *nothing* in the system will ever emit an increment for it — the reaper only expires existing rows. The counter will stay permanently off by one (or N, for N unpaired decrements). This is a real bug description, not a backstop. Consider rewording: "If the compensation also fails, the catalog's counter will be permanently off by one until an operator reconciles it — a known gap that a reconciliation job or outbox pattern would close." (The reaper is not a backstop for this specific case.) -->
<!-- [COPY EDIT] "DB down" — informal. "database down" is cleaner in a technical book. -->
<!-- [COPY EDIT] "context cancelled" — US "canceled" / UK "cancelled" — same consistency check as "serialises". -->

[toctou]: https://en.wikipedia.org/wiki/Time-of-check_to_time-of-use

This pattern is sometimes called "optimistic decrement with compensation" and is the pragmatic middle ground between a full two-phase commit (overkill here) and a distributed saga (useful when the workflow has more than two steps). The underlying principle — *let the database be the arbiter, not the application* — applies to any scarce-resource allocation: seat booking, inventory reservation, rate-limit token issuance.

<!-- [COPY EDIT] "two-phase commit" — lowercase, hyphenated. Consistent with 7.1. Good. -->
<!-- [STRUCTURAL] "optimistic decrement with compensation" in quotes — suggest italicizing as a pattern name and dropping the quotes. -->

### Returning a Book

The return flow is simpler -- verify ownership, check status, update, publish:

```go
func (s *ReservationService) ReturnBook(ctx context.Context, reservationID uuid.UUID) (*model.Reservation, error) {
    userID, err := pkgauth.UserIDFromContext(ctx)
    if err != nil {
        return nil, fmt.Errorf("user not authenticated: %w", err)
    }

    res, err := s.repo.GetByID(ctx, reservationID)
    if err != nil {
        return nil, err
    }

    if res.UserID != userID {
        return nil, model.ErrPermissionDenied
    }

    if res.Status != model.StatusActive {
        return nil, model.ErrAlreadyReturned
    }

    now := time.Now()
    res.Status = model.StatusReturned
    res.ReturnedAt = &now

    updated, err := s.repo.Update(ctx, res)
    if err != nil {
        return nil, fmt.Errorf("update reservation: %w", err)
    }

    if err := s.publisher.Publish(ctx, ReservationEvent{
        Type:          "reservation.returned",
        ReservationID: updated.ID.String(),
        UserID:        userID.String(),
        BookID:        updated.BookID.String(),
        Timestamp:     now,
    }); err != nil {
        slog.ErrorContext(ctx, "failed to publish event", ...)
    }

    return updated, nil
}
```

The ownership check (`res.UserID != userID`) is critical. Without it, any authenticated user could return anyone's reservation. This is a common security concern in multi-tenant systems -- always verify that the requesting user owns the resource they are acting on.

<!-- [STRUCTURAL] Good security callout. Consider bolding "IDOR" (Insecure Direct Object Reference) for readers who will encounter the term in OWASP docs: "This is a common security concern in multi-tenant systems — an Insecure Direct Object Reference (IDOR) vulnerability if skipped." -->
<!-- [COPY EDIT] "multi-tenant" — hyphenated compound adjective (CMOS 7.81). Correct. -->

### Expiring Reservations: Read Path vs. Background Reaper

<!-- [COPY EDIT] Heading: "Expiring Reservations: Read Path vs. Background Reaper" — after the colon, "Read Path vs. Background Reaper" is a sentence fragment; first word capitalized per CMOS 6.63 when using headline case. Good. -->

A reservation becomes *logically* expired the moment `time.Now()` passes `DueAt`, but the database row still says `status = 'active'` until something updates it. Two code paths do that work, and they exist for different reasons.

**Path 1: expire-on-read.** `GetReservation` and `ListUserReservations` call `expireIfDue` on every row they return, which flips any overdue row to `expired` before handing it back:

```go
func (s *ReservationService) expireIfDue(ctx context.Context, r *model.Reservation) {
    if r.Status != model.StatusActive || time.Now().Before(r.DueAt) {
        return
    }
    s.expireReservation(ctx, r)
}
```

This is **lazy evaluation**. It keeps what the user sees consistent with what the clock says: users never observe their own overdue reservations still listed as active, because the act of reading fixes the row on the way out.

<!-- [LINE EDIT] "users never observe their own overdue reservations still listed as active" → "users never see their own overdue reservations still listed as active" — "observe" is slightly formal; "see" matches the sentence preceding it. -->

**Path 2: the reaper.** The problem with _only_ doing expire-on-read is that if nobody reads a reservation — the user stopped logging in, the account was deleted, the request never happens — the row stays `active` forever. Worse, the catalog's `available_copies` never gets incremented back, so the book is permanently marked as held. To close this gap the reservation service runs a background goroutine that periodically finds and expires overdue rows:

<!-- [LINE EDIT] "is permanently marked as held" → "is permanently marked as checked out" — "held" is ambiguous in library terminology (hold/reservation vs. checked-out/loan). Optional. -->

```go
func (s *ReservationService) RunExpirationReaper(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.ReapExpired(ctx)
        }
    }
}

func (s *ReservationService) ReapExpired(ctx context.Context) {
    due, err := s.repo.ListDueForExpiration(ctx, time.Now())
    if err != nil {
        slog.ErrorContext(ctx, "reaper: list due reservations failed", "error", err)
        return
    }
    for _, r := range due {
        s.expireReservation(ctx, r)
    }
}
```

Both paths route through the same `expireReservation` helper, which flips the row, saves it, and publishes the `reservation.expired` event that the catalog's consumer picks up to increment availability. Keeping the write logic in one place means the two paths cannot disagree about what "expired" means or forget to publish.

<!-- [STRUCTURAL] Good DRY justification. Keep. -->

The reaper is wired into `cmd/main.go` as a goroutine that shares the service's signal-aware context, so it stops cleanly on `SIGTERM`:

```go
go reservationSvc.RunExpirationReaper(ctx, reaperInterval)
```

`REAPER_INTERVAL` defaults to 5 minutes. That is a deliberate tradeoff: the window during which a book can be stale on the catalog is roughly (`DueAt` to `DueAt + 5 minutes`), which is plenty for a library but would not suffice for, say, seat inventory on a flight. Tune it via the env var if you need tighter bounds — the cost is one full-table scan for active rows per tick.

<!-- [COPY EDIT] "5 minutes" — spell out "five minutes" in running prose (CMOS 9.2 says spell out numbers zero–ninety-nine in general contexts). However, for technical/measurement usage, numerals are acceptable. House style call. -->
<!-- [COPY EDIT] "env var" — informal. "environment variable" is clearer for a book. Use the spelled form on first use. -->
<!-- [COPY EDIT] "one full-table scan" — "full-table" hyphenated as compound modifier before "scan". CMOS 7.81. Correct. -->
<!-- [COPY EDIT] "(DueAt to DueAt + 5 minutes)" — the parenthetical is inside another parenthetical implicitly. Consider: "the window during which a book can be stale on the catalog is roughly DueAt to DueAt + 5 minutes — fine for a library, not enough for, say, seat inventory on a flight." -->

**Why both, not just the reaper?** The reaper fires on a timer, so between ticks a user could reload their reservations page and briefly see an overdue row still listed as active. That is a small but visible inconsistency that expire-on-read eliminates without needing a ≤ 1-second timer. The two mechanisms are complementary: read-triggered for user-facing freshness, time-triggered for catalog reconciliation and unread rows.

<!-- [COPY EDIT] "≤ 1-second timer" — fine; no hyphen needed between "≤" and number. "1-second" hyphenated as compound modifier. CMOS 7.81. -->

Note the defensive programming in `expireReservation`: if the database update fails, the method reverts the in-memory status change so the caller does not see stale data. And because the reaper runs with a background context that has no user attached, the helper falls back to the reservation's own `UserID` when publishing the event.

<!-- [LINE EDIT] "And because the reaper runs with a background context that has no user attached" → "Because the reaper's background context has no user attached" — Shorter, drops the leading "And". -->

---

## gRPC Handler

The handler layer translates between protobuf types and domain types. It follows the same pattern as the catalog handler from Chapter 2:

```go
// services/reservation/internal/handler/handler.go

type ReservationHandler struct {
    reservationv1.UnimplementedReservationServiceServer
    svc Service
}
```

The embedded `UnimplementedReservationServiceServer` provides default implementations for all RPC methods (returning "unimplemented" errors). This ensures forward compatibility -- if you add a new RPC to the proto file, the service compiles without implementing it immediately.

<!-- [COPY EDIT] "forward compatibility" — noun phrase, open. Correct. -->

The `Service` interface defines what the handler needs:

```go
type Service interface {
    CreateReservation(ctx context.Context, bookID uuid.UUID) (*model.Reservation, error)
    ReturnBook(ctx context.Context, reservationID uuid.UUID) (*model.Reservation, error)
    GetReservation(ctx context.Context, reservationID uuid.UUID) (*model.Reservation, error)
    ListUserReservations(ctx context.Context) ([]*model.Reservation, error)
}
```

This is a narrower interface than the full `ReservationService` struct -- it only exposes the methods the handler uses. In Go, it is idiomatic to define interfaces at the point of use, not at the point of implementation. The handler says "I need something that can do these four things" and does not care whether it is a real service, a mock, or a decorator.

<!-- [STRUCTURAL] "point of use / point of implementation" — great Go idiom explanation. Keep. -->

### Protobuf Mapping

The `reservationToProto` function converts the domain model to a protobuf message:

```go
func reservationToProto(r *model.Reservation) *reservationv1.Reservation {
    pb := &reservationv1.Reservation{
        Id:         r.ID.String(),
        UserId:     r.UserID.String(),
        BookId:     r.BookID.String(),
        Status:     r.Status,
        ReservedAt: timestamppb.New(r.ReservedAt),
        DueAt:      timestamppb.New(r.DueAt),
    }
    if r.ReturnedAt != nil {
        pb.ReturnedAt = timestamppb.New(*r.ReturnedAt)
    }
    return pb
}
```

UUIDs become strings, `time.Time` becomes `google.protobuf.Timestamp` (via `timestamppb.New`), and the nullable `ReturnedAt` is only set when non-nil. This boilerplate is the cost of keeping the domain model separate from the transport model -- but it means the domain model never depends on protobuf.

### Error Mapping

The `toGRPCError` function maps domain errors to gRPC status codes:

```go
func toGRPCError(err error) error {
    switch {
    case errors.Is(err, model.ErrReservationNotFound):
        return status.Error(codes.NotFound, err.Error())
    case errors.Is(err, model.ErrMaxReservations):
        return status.Error(codes.ResourceExhausted, err.Error())
    case errors.Is(err, model.ErrNoAvailableCopies):
        return status.Error(codes.FailedPrecondition, "no copies available")
    case errors.Is(err, model.ErrAlreadyReturned):
        return status.Error(codes.FailedPrecondition, err.Error())
    case errors.Is(err, model.ErrPermissionDenied):
        return status.Error(codes.PermissionDenied, "permission denied")
    default:
        return status.Error(codes.Internal, "internal error")
    }
}
```

This is a switch on sentinel errors using `errors.Is()`, which works correctly with wrapped errors (the `%w` verb in `fmt.Errorf`). The choice of gRPC codes matters for the gateway -- it maps them to HTTP status codes for the user. `ResourceExhausted` becomes 429, `FailedPrecondition` becomes 412, `PermissionDenied` becomes 403.

<!-- [COPY EDIT] Please verify: HTTP status code mapping conventions — gRPC's `ResourceExhausted` does map to HTTP 429 per the gRPC-HTTP mapping spec. `FailedPrecondition` → 400 in the HTTP mapping (not 412), per grpc.io and the generic gRPC→HTTP mapping table. 412 is used in some conventions, but the gRPC team's recommendation is 400. Verify against the gateway's actual `handleGRPCError` (section 7.4), which maps `FailedPrecondition` → `StatusPreconditionFailed` (412). If the gateway does use 412, this paragraph is correct for this codebase. Flag as a book-internal consistency check. -->
<!-- [COPY EDIT] Number usage for HTTP status codes — numerals are correct (CMOS 9.15). -->

The `default` case returns `codes.Internal` with a generic message. Never leak internal error details to clients -- they are a security risk and useless to end users. Log the full error server-side.

<!-- [LINE EDIT] "they are a security risk and useless to end users" → "they are a security risk and offer no value to end users" — Avoids "useless" which is informal; clarifies parallelism. Optional. -->

---

## Wiring in main.go

The `main.go` function follows the same pattern as every other service:

<!-- [COPY EDIT] "main.go function" — minor: `main.go` is the file; `main` is the function. Suggest: "The `main` function follows the same pattern as every other service:". -->

```go
// services/reservation/cmd/main.go

repo := repository.NewReservationRepository(db)
reservationSvc := service.NewReservationService(repo, catalogClient, publisher, maxActive)
reservationHandler := handler.NewReservationHandler(reservationSvc)
```

Three lines to wire the entire application: create the repository, create the service (injecting the repo, catalog client, event publisher, and config), create the handler (injecting the service). Every dependency is explicit. Compare this to Spring Boot, where the equivalent would be three `@Component` classes with `@Autowired` constructors, and the wiring would happen invisibly through component scanning.

<!-- [LINE EDIT] "Three lines to wire the entire application" — overstated; these three lines wire the core, but main.go also wires the DB connection, the gRPC listener, the catalog client, the publisher, etc. Suggest: "Three lines to wire the domain stack:". -->

The service also creates a gRPC connection to the catalog service, since it needs to check availability:

```go
catalogConn, err := grpc.NewClient(catalogAddr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
```

<!-- [COPY EDIT] "since it needs to check availability" — same framing issue: the gRPC call is a decrement, not a check. Suggest: "since it needs to reserve copies through the catalog." -->

The `otelgrpc.NewClientHandler()` adds OpenTelemetry instrumentation to outgoing gRPC calls, so the synchronous catalog lookup appears in the distributed trace.

<!-- [COPY EDIT] "synchronous catalog lookup" — same framing issue. Suggest: "synchronous catalog call". -->

---

## The Layered Pattern, Revisited

If you compare the reservation service to the catalog service from Chapter 2, the structure is identical:

| Layer | Catalog | Reservation |
|-------|---------|-------------|
| Model | `model.Book`, sentinel errors | `model.Reservation`, sentinel errors |
| Repository | `BookRepository` with GORM | `ReservationRepository` with GORM |
| Service | `CatalogService` with business rules | `ReservationService` with business rules |
| Handler | `CatalogHandler` with proto mapping | `ReservationHandler` with proto mapping |
| main.go | Wire everything, start gRPC server | Wire everything, start gRPC server |

The reservation service adds two things the catalog service did not have: a gRPC client dependency (calling catalog for availability checks) and an event publisher (sending events to Kafka). But the layering is the same. Each layer depends only on the layer below it (via interfaces), and the handler never touches the database directly.

<!-- [COPY EDIT] "calling catalog for availability checks" — same framing issue throughout. Normalize to "decrementing catalog availability" or "reserving copies against the catalog". -->

This consistency is the payoff of a well-chosen architecture. Once you understand one service, you understand them all. New team members can navigate unfamiliar services because the structure is predictable. This is not exciting -- it is the point.

<!-- [STRUCTURAL] Strong closing line. Keep. -->

---

## Exercises

1. **Add ExtendReservation.** Implement an `ExtendReservation` method that extends the `DueAt` by 7 days. Business rules: only the owner can extend, only active reservations can be extended, and a reservation can be extended at most once (add an `ExtendedAt *time.Time` field). Write the repository, service, and handler methods.

2. **Test the service layer.** Write a unit test for `CreateReservation` that uses a mock repository, a mock catalog client, and a mock event publisher. Test these cases: successful creation, max reservations exceeded, no available copies.

<!-- [STRUCTURAL] Good exercise, but given the TOCTOU section now dominates the design narrative, consider adding a test case: "and: catalog decrement succeeds but repo.Create fails — verify the compensation UpdateAvailability(+1) is called." That directly exercises the most subtle path. -->

3. **State machine diagram.** Draw a state machine diagram for the reservation lifecycle. Include all three states and the transitions between them. What triggers each transition?

4. **Alternatives to decrement-then-reserve.** The main text explains why we decrement availability first and compensate on failure. What other approaches could close the same TOCTOU gap? Sketch the pros and cons of (a) a full two-phase commit across catalog and reservation, (b) a Saga pattern with explicit compensating transactions, (c) optimistic concurrency with a version column on `books`, and (d) a single cross-service transactional outbox. For each, identify a scenario where it would outperform the current design.

<!-- [STRUCTURAL] Meaty exercise; good depth. Keep. -->
<!-- [COPY EDIT] "Saga pattern" — proper pattern name, capitalized. Good. -->

5. **Reaper durability.** The reaper in `RunExpirationReaper` runs in-process: if the only reservation replica crashes between ticks, overdue rows linger until it restarts. Sketch two ways to make expiration durable against crashes — (a) moving the reaper into a dedicated cron pod / Kubernetes `CronJob`, and (b) pushing expiration into PostgreSQL itself via a scheduled `UPDATE` in `pg_cron`. What are the operational tradeoffs? Consider visibility, retries, and who owns the compensation logic when the expire event fails to publish.

<!-- [COPY EDIT] "cron pod / Kubernetes CronJob" — slash OK; "Kubernetes CronJob" capitalized as the resource type is correct. -->
<!-- [COPY EDIT] "pg_cron" — code-font for the extension name. Correct. -->

---

## References

[^1]: [GORM documentation](https://gorm.io/docs/) -- Official documentation for the GORM ORM library.
[^2]: [gRPC Go quickstart](https://grpc.io/docs/languages/go/quickstart/) -- Getting started with gRPC in Go.
[^3]: [google.golang.org/protobuf/types/known/timestamppb](https://pkg.go.dev/google.golang.org/protobuf/types/known/timestamppb) -- Protobuf Timestamp helper for Go.
[^4]: [Go interfaces: implicit satisfaction](https://go.dev/tour/methods/10) -- How Go interfaces work without explicit `implements` declarations.
[^5]: [errors.Is and errors.As](https://pkg.go.dev/errors#Is) -- Go standard library error matching, used for sentinel error checks.

<!-- [COPY EDIT] " -- " in reference list entries — normalize chapter-wide to em dash or period. (CMOS 6.85) -->
<!-- [COPY EDIT] Reference list is light on the TOCTOU/saga/outbox topic given how central those are to this section. Consider adding: (a) the Wikipedia TOCTOU article already linked inline; (b) Chris Richardson's Saga pattern page (microservices.io/patterns/data/saga.html); (c) a PostgreSQL docs pointer for row-level locking on UPDATE. -->
<!-- [FINAL] Double-check: the file path comments (`// services/reservation/internal/model/model.go` etc.) — verify these paths match the current repo layout. -->
