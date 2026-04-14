# 7.2 Reservation Service

The reservation service is the second microservice we build from scratch. If you followed Chapter 2 (the catalog service), this one will feel familiar -- same layered architecture, same patterns. That repetition is deliberate. The goal is to show that the patterns are general, not specific to any one domain. Once you internalize the model/repository/service/handler stack, you can stand up a new service quickly.

The interesting differences are in the domain logic: state machines, cross-service reads, event publishing, and a two-pronged expiration strategy — lazy expire-on-read for responsiveness plus a background reaper for timeliness.

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

**`ReturnedAt` is a pointer.** A `*time.Time` is Go's way of expressing a nullable timestamp. When the reservation is active, `ReturnedAt` is `nil`. When returned, it is set. GORM understands pointer fields as nullable columns. In Kotlin, you would write `val returnedAt: Instant?` -- same concept, different syntax.

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

---

## Service Layer

The service layer is where the interesting logic lives. Let us look at its dependencies:

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

3. **`publisher` is an interface.** The `EventPublisher` interface has one method: `Publish(ctx, event) error`. The Kafka publisher implements it, but in tests you can substitute a mock. This is the same dependency inversion pattern used throughout the codebase.

### Creating a Reservation

The `CreateReservation` method enforces all the business rules. The interesting part is the order of operations — we decrement availability **before** creating the reservation row, not after. The next section (_The TOCTOU trap_) explains why.

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

The user ID comes from the context via `pkgauth.UserIDFromContext`. The auth middleware (a gRPC interceptor in this case) validates the JWT token and injects the user ID into the context before the handler runs. This is the same pattern the gateway uses -- extract auth info from the context, not from function parameters.

The loan duration is a package-level constant: `const loanDuration = 14 * 24 * time.Hour`. This is idiomatic Go -- constants for configuration that does not change at runtime. If this needed to be configurable per environment, it would move to a constructor parameter (like `maxActive`).

### The TOCTOU trap (and why we decrement first)

An earlier version of `CreateReservation` looked natural:

```go
book, _ := s.catalog.GetBook(ctx, ...)
if book.AvailableCopies <= 0 {
    return nil, model.ErrNoAvailableCopies
}
// ...create reservation, then later somebody decrements availability...
```

It is also **wrong** under concurrency. This is a textbook [Time-Of-Check-to-Time-Of-Use][toctou] bug: two requests for the last copy of a book call `GetBook` in parallel, both see `AvailableCopies == 1`, both pass the guard, both create reservations. The catalog ends up with `available_copies = -1` or, worse, two users hold the same physical copy.

The fix flips the flow so that the **database** is the gate, not the service:

1. `catalog.UpdateAvailability(book, -1)` runs a guarded `UPDATE` that refuses to go below zero (`WHERE available_copies + ? >= 0`). PostgreSQL's row-level locking during `UPDATE` serialises the two racing decrements — one wins, the other gets zero rows affected and returns `FailedPrecondition`.
2. Only after the decrement succeeds do we create the reservation row.
3. If the reservation insert then fails (DB down, constraint violation, context cancelled), we compensate with `UpdateAvailability(+1)` so catalog's counter does not drift. The compensation is best-effort — if it also fails, the expiration reaper (see _Expiring reservations_) provides a backstop, since an unpaired decrement will eventually be reconciled when other reservations expire and the numbers converge.

This pattern is sometimes called "optimistic decrement with compensation" and is the pragmatic middle ground between a full two-phase commit (overkill here) and a distributed saga (useful when the workflow has more than two steps). The underlying principle — *let the database be the arbiter, not the application* — applies to any scarce-resource allocation: seat booking, inventory reservation, rate-limit token issuance.

[toctou]: https://en.wikipedia.org/wiki/Time-of-check_to_time-of-use

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

### Expiring Reservations: Read Path vs. Background Reaper

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

**Path 2: the reaper.** The problem with _only_ doing expire-on-read is that if nobody reads a reservation — the user stopped logging in, the account was deleted, the request never happens — the row stays `active` forever. Worse, the catalog's `available_copies` never gets incremented back, so the book is permanently marked as held. To close this gap the reservation service runs a background goroutine that periodically finds and expires overdue rows:

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

The reaper is wired into `cmd/main.go` as a goroutine that shares the service's signal-aware context, so it stops cleanly on `SIGTERM`:

```go
go reservationSvc.RunExpirationReaper(ctx, reaperInterval)
```

`REAPER_INTERVAL` defaults to 5 minutes. That is a deliberate tradeoff: the window during which a book can be stale on the catalog is roughly (`DueAt` to `DueAt + 5 minutes`), which is plenty for a library but would not suffice for, say, seat inventory on a flight. Tune it via the env var if you need tighter bounds — the cost is one full-table scan for active rows per tick.

**Why both, not just the reaper?** The reaper fires on a timer, so between ticks a user could reload their reservations page and briefly see an overdue row still listed as active. That is a small but visible inconsistency that expire-on-read eliminates without needing a ≤ 1-second timer. The two mechanisms are complementary: read-triggered for user-facing freshness, time-triggered for catalog reconciliation and unread rows.

Note the defensive programming in `expireReservation`: if the database update fails, the method reverts the in-memory status change so the caller does not see stale data. And because the reaper runs with a background context that has no user attached, the helper falls back to the reservation's own `UserID` when publishing the event.

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

The `default` case returns `codes.Internal` with a generic message. Never leak internal error details to clients -- they are a security risk and useless to end users. Log the full error server-side.

---

## Wiring in main.go

The `main.go` function follows the same pattern as every other service:

```go
// services/reservation/cmd/main.go

repo := repository.NewReservationRepository(db)
reservationSvc := service.NewReservationService(repo, catalogClient, publisher, maxActive)
reservationHandler := handler.NewReservationHandler(reservationSvc)
```

Three lines to wire the entire application: create the repository, create the service (injecting the repo, catalog client, event publisher, and config), create the handler (injecting the service). Every dependency is explicit. Compare this to Spring Boot, where the equivalent would be three `@Component` classes with `@Autowired` constructors, and the wiring would happen invisibly through component scanning.

The service also creates a gRPC connection to the catalog service, since it needs to check availability:

```go
catalogConn, err := grpc.NewClient(catalogAddr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
```

The `otelgrpc.NewClientHandler()` adds OpenTelemetry instrumentation to outgoing gRPC calls, so the synchronous catalog lookup appears in the distributed trace.

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

This consistency is the payoff of a well-chosen architecture. Once you understand one service, you understand them all. New team members can navigate unfamiliar services because the structure is predictable. This is not exciting -- it is the point.

---

## Exercises

1. **Add ExtendReservation.** Implement an `ExtendReservation` method that extends the `DueAt` by 7 days. Business rules: only the owner can extend, only active reservations can be extended, and a reservation can be extended at most once (add an `ExtendedAt *time.Time` field). Write the repository, service, and handler methods.

2. **Test the service layer.** Write a unit test for `CreateReservation` that uses a mock repository, a mock catalog client, and a mock event publisher. Test these cases: successful creation, max reservations exceeded, no available copies.

3. **State machine diagram.** Draw a state machine diagram for the reservation lifecycle. Include all three states and the transitions between them. What triggers each transition?

4. **Alternatives to decrement-then-reserve.** The main text explains why we decrement availability first and compensate on failure. What other approaches could close the same TOCTOU gap? Sketch the pros and cons of (a) a full two-phase commit across catalog and reservation, (b) a Saga pattern with explicit compensating transactions, (c) optimistic concurrency with a version column on `books`, and (d) a single cross-service transactional outbox. For each, identify a scenario where it would outperform the current design.

5. **Reaper durability.** The reaper in `RunExpirationReaper` runs in-process: if the only reservation replica crashes between ticks, overdue rows linger until it restarts. Sketch two ways to make expiration durable against crashes — (a) moving the reaper into a dedicated cron pod / Kubernetes `CronJob`, and (b) pushing expiration into PostgreSQL itself via a scheduled `UPDATE` in `pg_cron`. What are the operational tradeoffs? Consider visibility, retries, and who owns the compensation logic when the expire event fails to publish.

---

## References

[^1]: [GORM documentation](https://gorm.io/docs/) -- Official documentation for the GORM ORM library.
[^2]: [gRPC Go quickstart](https://grpc.io/docs/languages/go/quickstart/) -- Getting started with gRPC in Go.
[^3]: [google.golang.org/protobuf/types/known/timestamppb](https://pkg.go.dev/google.golang.org/protobuf/types/known/timestamppb) -- Protobuf Timestamp helper for Go.
[^4]: [Go interfaces: implicit satisfaction](https://go.dev/tour/methods/10) -- How Go interfaces work without explicit `implements` declarations.
[^5]: [errors.Is and errors.As](https://pkg.go.dev/errors#Is) -- Go standard library error matching, used for sentinel error checks.
