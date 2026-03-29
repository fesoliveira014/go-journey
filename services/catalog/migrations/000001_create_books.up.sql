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
