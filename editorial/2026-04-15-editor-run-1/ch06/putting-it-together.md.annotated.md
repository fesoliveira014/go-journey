# 6.4 Putting It Together

<!-- [STRUCTURAL] This is exactly the right shape for a capstone section: numbered, prescriptive, verifiable. Each step ends with an expected visible outcome. Pacing is excellent. -->

<!-- [LINE EDIT] "This section walks through the entire flow end-to-end: starting the stack, bootstrapping an admin, seeding the catalog, and verifying everything through the UI. Follow these steps in order." — Good. Keep. -->
This section walks through the entire flow end-to-end: starting the stack, bootstrapping an admin, seeding the catalog, and verifying everything through the UI. Follow these steps in order.
<!-- [COPY EDIT] "end-to-end" — compound adjective before noun; hyphenated. CMOS 7.89. Correct. -->
<!-- [COPY EDIT] CMOS 6.19 serial comma: four-item list of gerund phrases; correct. -->

---

## Step 1: Start the Stack
<!-- [COPY EDIT] CMOS 6.63 heading: "Step 1: Start the Stack" — the portion after the colon is a complete imperative; initial caps appropriate. -->

```bash
cd deploy && docker compose up --build
```

<!-- [LINE EDIT] "Wait for all services to be healthy. You should see log output from `auth`, `catalog`, `reservation`, `gateway`, and the two Postgres instances. The gateway will be available at `http://localhost:8080`." — Good. Keep. -->
Wait for all services to be healthy. You should see log output from `auth`, `catalog`, `reservation`, `gateway`, and the two Postgres instances. The gateway will be available at `http://localhost:8080`.
<!-- [COPY EDIT] "Postgres" — informal for "PostgreSQL". Both forms are used elsewhere in the chapter (full form in `admin-cli.md`). Consistency: recommend "PostgreSQL" for the first mention per chapter and "Postgres" thereafter, or standardize on one form. -->
<!-- [COPY EDIT] CMOS 6.19 serial comma: "`auth`, `catalog`, `reservation`, `gateway`, and the two Postgres instances" — correct. -->
<!-- [STRUCTURAL] Minor: what does "healthy" mean operationally? Could the author point to `docker compose ps` showing "healthy" status, or to log lines that signal readiness? Readers unfamiliar with Docker Compose health checks may wait indefinitely. One-line addendum would help. -->

---

## Step 2: Create an Admin Account

In a new terminal, run the admin CLI:

```bash
DATABASE_URL="postgres://postgres:postgres@localhost:5434/auth?sslmode=disable" \
  go run ./services/auth/cmd/admin \
    --email admin@library.local \
    --password admin123 \
    --name "Library Admin"
```

You should see:

```
Created admin user: admin@library.local (some-uuid-here)
```

<!-- [LINE EDIT] "The connection string uses port `5434` because that is the host port mapped to the auth database in `deploy/.env` (`POSTGRES_AUTH_PORT=5434`). The credentials (`postgres:postgres`) and database name (`auth`) are also defined there." — Good. Keep. -->
The connection string uses port `5434` because that is the host port mapped to the auth database in `deploy/.env` (`POSTGRES_AUTH_PORT=5434`). The credentials (`postgres:postgres`) and database name (`auth`) are also defined there.

---

## Step 3: Seed the Catalog

```bash
go run ./services/catalog/cmd/seed \
  --email admin@library.local \
  --password admin123
```

<!-- [LINE EDIT] "You should see 16 books created. The seed CLI connects to `localhost:50051` (auth) and `localhost:50052` (catalog) by default, which match the ports exposed by Docker Compose." — Good. Keep. -->
You should see 16 books created. The seed CLI connects to `localhost:50051` (auth) and `localhost:50052` (catalog) by default, which match the ports exposed by Docker Compose.
<!-- [COPY EDIT] Port numbers consistent with seed-cli.md; the only discrepancy in the chapter is admin-dashboard.md's reference to 50053 for the reservation service. See query in that file. -->

---

## Step 4: Log In as Admin
<!-- [COPY EDIT] "Log In" as heading (two words) — correct as phrasal verb. When used attributively ("login page"), close up. CMOS 7.89. -->

Open `http://localhost:8080/login` in your browser. Enter the admin credentials:

- **Email:** `admin@library.local`
- **Password:** `admin123`

After login, you should be redirected to the home page. The navigation bar should now show an **Admin** link (visible only to admin users).

---

## Step 5: Browse the Catalog

Navigate to `http://localhost:8080/books`. You should see all 16 seeded books listed with their titles, authors, and genres. Click on any book to see its details page, including available copies.
<!-- [COPY EDIT] CMOS 6.19 serial comma: "titles, authors, and genres" — correct. -->

---

## Step 6: Check the Admin Dashboard

Click the **Admin** link in the navigation, or go to `http://localhost:8080/admin`. You should see three cards:

- **Users** -- links to `/admin/users`
- **Reservations** -- links to `/admin/reservations`
- **Books** -- links to the "Add Book" form

Click **View Users**. You should see a table with at least one row: the admin account you just created, with role `admin`.
<!-- [LINE EDIT] "with at least one row: the admin account you just created, with role `admin`." → "with one row: the admin account you just created, with role `admin`." Reason: "just" is on the cut-list, and "at least one" is hedging — at this point in the walkthrough there is exactly one user. -->

Click **View Reservations**. The table should be empty -- no one has reserved any books yet.

---

## Step 7: Register a Regular User and Make a Reservation

1. **Log out** (or open a private/incognito window).
<!-- [COPY EDIT] "Log out" — two-word verb; correct. "logout" would be the noun/adjective form. -->
<!-- [COPY EDIT] "private/incognito" — the slash is acceptable for alternatives; CMOS 6.106 prefers "or" in formal prose. Register-appropriate here. -->
2. Go to `http://localhost:8080/register` and create a new account:
   - **Email:** `reader@example.com`
   - **Password:** `reader123`
   - **Name:** `Regular Reader`
3. After registration, you will be logged in automatically.
4. Navigate to the catalog (`/books`), click on a book (e.g., *Dune*), and click **Reserve**.
<!-- [COPY EDIT] CMOS 6.43: "(e.g., *Dune*)" — comma after "e.g." correct. Italics on book title correct per CMOS 8.171. -->
5. Go to `http://localhost:8080/reservations` to confirm the reservation appears in your list.

---

## Step 8: Verify in the Admin Dashboard

1. Log out of the regular user account.
2. Log back in as `admin@library.local`.
3. Go to `http://localhost:8080/admin/users`. You should now see two rows: the admin and the regular user.
4. Go to `http://localhost:8080/admin/reservations`. You should see one row showing:
   - **User:** `reader@example.com`
   - **Book:** *Dune* (or whichever book was reserved)
   - **Status:** `active`
   - **Reserved:** today's date
   - **Returned:** a dash (not yet returned)

<!-- [LINE EDIT] "This confirms the full flow: the reservation service resolves the book title from the catalog service and the user email from the auth service, embedding both in the `ReservationDetail` response." — Good. Keep. -->
This confirms the full flow: the reservation service resolves the book title from the catalog service and the user email from the auth service, embedding both in the `ReservationDetail` response.

---

## What's Next

<!-- [STRUCTURAL] Strong chapter-close. Names the limitation of the current design (synchronous gRPC, search service disconnected) and points to Chapter 7's payoff (decoupling via events). This is the right place to foreshadow without overselling. -->

At this point the library system is functional: users can register, browse, and reserve books; admins can manage the catalog and monitor activity. But the services are mostly isolated -- the reservation service calls catalog and auth synchronously via gRPC, and the search service is not yet connected.
<!-- [COPY EDIT] CMOS 6.19 serial comma: "register, browse, and reserve books" — correct. -->
<!-- [LINE EDIT] "But the services are mostly isolated" — factually mild. The reservation service *does* couple to catalog and auth synchronously; "isolated" may read as the opposite of what the author intends. Consider: "But the services are tightly coupled" or "But the services talk to each other synchronously, which is a limitation." -->

<!-- [LINE EDIT] "In **Chapter 7**, we introduce **Kafka** and **event-driven architecture**. The catalog service will publish `book.created`, `book.updated`, and `book.deleted` events. The search service will consume these events to build and maintain a search index. The reservation service will publish `reservation.created` and `reservation.returned` events that the catalog service consumes to update available copy counts." — 60 words across two sentences; paced well. Keep. -->
In **Chapter 7**, we introduce **Kafka** and **event-driven architecture**. The catalog service will publish `book.created`, `book.updated`, and `book.deleted` events. The search service will consume these events to build and maintain a search index. The reservation service will publish `reservation.created` and `reservation.returned` events that the catalog service consumes to update available copy counts.
<!-- [COPY EDIT] CMOS 6.19 serial comma: both event-name lists correct. -->
<!-- [COPY EDIT] "event-driven architecture" — compound adjective + noun; hyphenated. CMOS 7.89. Correct. -->

<!-- [LINE EDIT] "This shift from synchronous RPC to asynchronous events is where the microservices architecture starts to pay for its complexity -- services become more decoupled, more resilient, and more independently deployable." — Clean closing sentence. Keep. -->
This shift from synchronous RPC to asynchronous events is where the microservices architecture starts to pay for its complexity -- services become more decoupled, more resilient, and more independently deployable.
<!-- [COPY EDIT] "pay for its complexity" — idiomatic; fine. An alternate would be "pay off its complexity budget" but the current phrasing is stronger. -->
<!-- [COPY EDIT] CMOS 6.19 serial comma: "more decoupled, more resilient, and more independently deployable" — correct. -->
<!-- [FINAL] No typos, doubled words, or missing words found. Chapter ends strongly. -->
