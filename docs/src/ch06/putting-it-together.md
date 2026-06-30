# 6.4 Putting It Together

This section walks through the Chapter 6 checkpoint flow end-to-end: starting the stack, bootstrapping an admin, seeding the catalog, registering a normal user, and verifying the admin dashboard.

---

## Step 1: Start the Stack

```bash
cd deploy && docker compose up --build
```

Wait for the Chapter 6 services to be healthy. You should see log output from `auth`, `catalog`, `gateway`, and the PostgreSQL instances. The gateway will be available at `http://localhost:8080`.

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

You should see sixteen books created. The seed CLI connects to `localhost:50051` (auth) and `localhost:50052` (catalog) by default, which match the ports exposed by Docker Compose.

---

## Step 4: Log In as Admin

Open `http://localhost:8080/login` in your browser. Enter the admin credentials:

- **Email:** `admin@library.local`
- **Password:** `admin123`

After login, you should be redirected to the home page. The navigation bar should now show an **Admin** link (visible only to admin users).

---

## Step 5: Browse the Catalog

Navigate to `http://localhost:8080/books`. You should see all sixteen seeded books listed with their titles, authors, and genres. Click on any book to see its details page, including available copies.

---

## Step 6: Check the Admin Dashboard

Click the **Admin** link in the navigation, or go to `http://localhost:8080/admin`. You should see dashboard links for:

- **Users**—links to `/admin/users`
- **Books**—links to the "Add Book" form

Click **View Users**. You should see a table with one row: the admin account you just created, with role `admin`.

---

## Step 7: Register a Regular User

1. **Log out** (or open a private/incognito window).
2. Go to `http://localhost:8080/register` and create a new account:
   - **Email:** `reader@example.com`
   - **Password:** `reader123`
   - **Name:** `Regular Reader`
3. After registration, you will be logged in automatically.

---

## Step 8: Verify the User List

1. Log out of the regular user account.
2. Log back in as `admin@library.local`.
3. Go to `http://localhost:8080/admin/users`.

You should now see two rows: the admin and the regular user. This confirms the Chapter 6 flow: the admin CLI created an administrative account, the seed CLI populated the catalog through service APIs, and the dashboard can inspect users through an admin-only Auth RPC.

Chapter 7 adds reservations and then extends this dashboard with `/admin/reservations`.

---

## What's Next

At this point the development loop is much smoother: you can create an admin, seed catalog data, browse books, and inspect users without manual database edits. Users cannot reserve books yet because there is no Reservation service.

In **Chapter 7**, we introduce **Kafka** and **event-driven architecture**. The Reservation Service will call Catalog synchronously for availability changes, then publish `reservation.created` and `reservation.returned` events as facts for downstream observers. In Chapter 8, the Catalog Service will publish `book.created`, `book.updated`, and `book.deleted` events that the Search Service consumes to build and maintain a search index.

This shift from synchronous RPC to asynchronous events is where the microservices architecture starts to pay for its complexity—services become more decoupled, more resilient, and more independently deployable.
