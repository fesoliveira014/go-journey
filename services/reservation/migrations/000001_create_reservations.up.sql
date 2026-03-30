CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE reservations (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL,
    book_id     UUID NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'active',
    reserved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    due_at      TIMESTAMPTZ NOT NULL,
    returned_at TIMESTAMPTZ,
    CONSTRAINT valid_status CHECK (status IN ('active', 'returned', 'expired'))
);

CREATE INDEX idx_reservations_user_status ON reservations(user_id, status);
CREATE INDEX idx_reservations_book_status ON reservations(book_id, status);
