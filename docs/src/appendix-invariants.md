# Appendix: System Invariants

The implementation changes across chapters, but these invariants should remain true. When a chapter introduces a new service or transport, use this list to check whether the architecture still has one clear owner for each piece of state.

## Catalog and Availability

- `available_copies` must never be below `0`.
- `available_copies` must never exceed `total_copies`.
- A book with checked-out copies cannot be deleted.
- Catalog is the source of truth for books and availability.

## Reservations

- Reservation is the source of truth for reservation records.
- Reservation state transitions are one-way: `active -> returned` or `active -> expired`.
- Only the owning user can return a reservation.
- Reservation calls Catalog synchronously to claim or release availability; reservation events are facts for downstream observers, not inventory commands.

## Auth and Permissions

- Auth is the source of truth for users.
- Only admins can mutate catalog records.
- Service-to-service mutations require service identity, not just a user JWT.

## Projections

- Search is a projection and can be rebuilt from Catalog.
- Kafka propagates facts to consumers; it should not create a second owner for the same state.
- Consumers must tolerate duplicate delivery and unknown future event types.
