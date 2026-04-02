# 6.4 Putting It Together

This section walks through the entire flow end-to-end: starting the stack, bootstrapping an admin, seeding the catalog, and verifying everything through the UI. Follow these steps in order.

---

## Step 1: Start the Stack

```bash
cd deploy && docker compose up --build
```

Wait for all services to be healthy. You should see log output from `auth`, `catalog`, `reservation`, `gateway`, and the two Postgres instances. The gateway will be available at `http://localhost:8080`.

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

The connection string uses port `5434` because that is the host port mapped to the auth database in `deploy/.env` (`POSTGRES_AUTH_PORT=5434`). The credentials (`postgres:postgres`) and database name (`auth`) are also defined there.

---

## Step 3: Seed the Catalog

```bash
go run ./services/catalog/cmd/seed \
  --email admin@library.local \
  --password admin123
```

You should see 16 books created. The seed CLI connects to `localhost:50051` (auth) and `localhost:50052` (catalog) by default, which match the ports exposed by Docker Compose.

---

## Step 4: Log In as Admin

Open `http://localhost:8080/login` in your browser. Enter the admin credentials:

- **Email:** `admin@library.local`
- **Password:** `admin123`

After login, you should be redirected to the home page. The navigation bar should now show an **Admin** link (visible only to admin users).

---

## Step 5: Browse the Catalog

Navigate to `http://localhost:8080/books`. You should see all 16 seeded books listed with their titles, authors, and genres. Click on any book to see its details page, including available copies.

---

## Step 6: Check the Admin Dashboard

Click the **Admin** link in the navigation, or go to `http://localhost:8080/admin`. You should see three cards:

- **Users** -- links to `/admin/users`
- **Reservations** -- links to `/admin/reservations`
- **Books** -- links to the "Add Book" form

Click **View Users**. You should see a table with at least one row: the admin account you just created, with role `admin`.

Click **View Reservations**. The table should be empty -- no one has reserved any books yet.

---

## Step 7: Register a Regular User and Make a Reservation

1. **Log out** (or open a private/incognito window).
2. Go to `http://localhost:8080/register` and create a new account:
   - **Email:** `reader@example.com`
   - **Password:** `reader123`
   - **Name:** `Regular Reader`
3. After registration, you will be logged in automatically.
4. Navigate to the catalog (`/books`), click on a book (e.g., *Dune*), and click **Reserve**.
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

This confirms the full flow: the reservation service resolves the book title from the catalog service and the user email from the auth service, embedding both in the `ReservationDetail` response.

---

## What's Next

At this point the library system is functional: users can register, browse, and reserve books; admins can manage the catalog and monitor activity. But the services are mostly isolated -- the reservation service calls catalog and auth synchronously via gRPC, and the search service is not yet connected.

In **Chapter 7**, we introduce **Kafka** and **event-driven architecture**. The catalog service will publish `book.created`, `book.updated`, and `book.deleted` events. The search service will consume these events to build and maintain a search index. The reservation service will publish `reservation.created` and `reservation.returned` events that the catalog service consumes to update available copy counts.

This shift from synchronous RPC to asynchronous events is where the microservices architecture starts to pay for its complexity -- services become more decoupled, more resilient, and more independently deployable.
