# 6.2 Reservation Service

The reservation service is the second microservice we build from scratch. If you followed Chapter 2 (the catalog service), this one will feel familiar -- same layered architecture, same patterns. That repetition is deliberate. The goal is to show that the patterns are general, not specific to any one domain. Once you internalize the model/repository/service/handler stack, you can stand up a new service quickly.

The interesting differences are in the domain logic: state machines, cross-service reads, event publishing, and a "lazy expiration" pattern that avoids background workers entirely.

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

The `CreateReservation` method enforces all the business rules:

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

    // Rule 2: check book availability via the catalog service (sync gRPC call)
    book, err := s.catalog.GetBook(ctx, &catalogv1.GetBookRequest{Id: bookID.String()})
    if err != nil {
        return nil, fmt.Errorf("check book availability: %w", err)
    }
    if book.AvailableCopies <= 0 {
        return nil, model.ErrNoAvailableCopies
    }

    // Create the reservation
    now := time.Now()
    res := &model.Reservation{
        UserID:     userID,
        BookID:     bookID,
        Status:     model.StatusActive,
        ReservedAt: now,
        DueAt:      now.Add(loanDuration), // 14 days
    }
    created, err := s.repo.Create(ctx, res)
    if err != nil {
        return nil, fmt.Errorf("create reservation: %w", err)
    }

    // Publish event (fire and log on failure)
    if err := s.publisher.Publish(ctx, ReservationEvent{
        Type:          "reservation.created",
        ReservationID: created.ID.String(),
        UserID:        userID.String(),
        BookID:        bookID.String(),
        Timestamp:     now,
    }); err != nil {
        slog.ErrorContext(ctx, "failed to publish event", ...)
    }

    return created, nil
}
```

The user ID comes from the context via `pkgauth.UserIDFromContext`. The auth middleware (a gRPC interceptor in this case) validates the JWT token and injects the user ID into the context before the handler runs. This is the same pattern the gateway uses -- extract auth info from the context, not from function parameters.

The loan duration is a package-level constant: `const loanDuration = 14 * 24 * time.Hour`. This is idiomatic Go -- constants for configuration that does not change at runtime. If this needed to be configurable per environment, it would move to a constructor parameter (like `maxActive`).

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

### Expiration on Read

The expiration logic is interesting because there is no background worker:

```go
func (s *ReservationService) expireIfDue(ctx context.Context, r *model.Reservation) {
    if r.Status != model.StatusActive || time.Now().Before(r.DueAt) {
        return
    }

    r.Status = model.StatusExpired
    if _, err := s.repo.Update(ctx, r); err != nil {
        slog.ErrorContext(ctx, "failed to expire reservation", ...)
        r.Status = model.StatusActive // revert in-memory change
        return
    }

    // Publish reservation.expired event
    if err := s.publisher.Publish(ctx, ReservationEvent{
        Type:          "reservation.expired",
        // ...
    }); err != nil {
        slog.ErrorContext(ctx, "failed to publish event", ...)
    }
}
```

This method is called during reads -- `GetReservation` and `ListUserReservations` both call it. When you fetch a reservation, the service checks whether its due date has passed. If so, it transitions the status to `expired` and publishes the event.

This is sometimes called **lazy evaluation** or **expiration on read**. The advantages are:

- No background goroutine or cron job to manage
- No race conditions between a background worker and request handlers
- Expiration only happens when someone actually looks at the data

The disadvantage is that a reservation might be logically expired but still show as `active` in the database until someone reads it. For a library system, this is fine -- the worst case is that the availability count is off by one for a brief period. For a financial system, you would need a background process.

Note the defensive programming: if the database update fails, the method reverts the in-memory status change (`r.Status = model.StatusActive`) so the caller does not see stale data.

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

4. **Race condition analysis.** Two users try to reserve the last copy of a book simultaneously. Both pass the availability check (`book.AvailableCopies > 0`). Both create reservations. The catalog ends up with `available_copies = -1`. How would you prevent this? Consider database-level locking, optimistic concurrency, or the Saga pattern.

5. **Background expiration.** Rewrite the expiration logic as a background goroutine that runs every minute and expires overdue reservations. What are the tradeoffs compared to the expiration-on-read approach?

---

## References

[^1]: [GORM documentation](https://gorm.io/docs/) -- Official documentation for the GORM ORM library.
[^2]: [gRPC Go quickstart](https://grpc.io/docs/languages/go/quickstart/) -- Getting started with gRPC in Go.
[^3]: [google.golang.org/protobuf/types/known/timestamppb](https://pkg.go.dev/google.golang.org/protobuf/types/known/timestamppb) -- Protobuf Timestamp helper for Go.
[^4]: [Go interfaces: implicit satisfaction](https://go.dev/tour/methods/10) -- How Go interfaces work without explicit `implements` declarations.
[^5]: [errors.Is and errors.As](https://pkg.go.dev/errors#Is) -- Go standard library error matching, used for sentinel error checks.
