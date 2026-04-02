# 6.3 Catalog Seed CLI

## The Problem

An empty catalog is useless for development. You need books to browse, reserve, and test against. Typing them in through the admin UI one at a time is slow and inconsistent -- every developer on the project would create different test data, making it impossible to reproduce bugs or write meaningful integration tests.

The solution is a **seed CLI**: a tool that loads a predefined set of books from a JSON fixture file and creates them through the catalog service's gRPC API.

---

## Why gRPC Seeding (Not Direct DB Insert)?

The admin CLI from Section 6.1 connects directly to the database. The seed CLI does not -- it authenticates as an admin and calls `CreateBook` through gRPC. This is a deliberate design choice:

- **Exercises the full stack.** The seed process goes through authentication (login via auth service), authorization (the JWT interceptor checks the admin role), validation (the catalog handler rejects missing titles or duplicate ISBNs), and event publishing (if Kafka is configured, `book.created` events are emitted).
- **Catches integration bugs.** If the auth service is misconfigured, or the catalog's JWT interceptor rejects the token, or a validation rule is wrong, the seed CLI will fail -- giving you early feedback.
- **Mirrors real usage.** An admin adding books through the UI follows the same code path. The seed CLI is just an automated version of that workflow.

The admin CLI bypasses gRPC because no gRPC endpoint exists for its operation. The seed CLI uses gRPC because `CreateBook` already exists and works correctly.

---

## Code Walkthrough

### Struct and Imports

```go
// services/catalog/cmd/seed/main.go

type seedBook struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	ISBN          string `json:"isbn"`
	Genre         string `json:"genre"`
	Description   string `json:"description"`
	PublishedYear int32  `json:"published_year"`
	TotalCopies   int32  `json:"total_copies"`
}
```

The `seedBook` struct mirrors the `CreateBookRequest` proto fields. It uses `json` struct tags for deserialization from the fixture file. Note the `int32` types for year and copies -- these match the protobuf field types, avoiding a conversion step.

### Flag Parsing and JSON Loading

```go
func main() {
	authAddr := flag.String("auth-addr", "localhost:50051", "auth service gRPC address")
	catalogAddr := flag.String("catalog-addr", "localhost:50052", "catalog service gRPC address")
	email := flag.String("email", "", "admin email (required)")
	password := flag.String("password", "", "admin password (required)")
	booksFile := flag.String("books", "services/catalog/cmd/seed/books.json", "path to books JSON file")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: seed --email EMAIL --password PASSWORD [--auth-addr ADDR] [--catalog-addr ADDR] [--books FILE]")
		os.Exit(1)
	}

	data, err := os.ReadFile(*booksFile)
	if err != nil {
		log.Fatalf("failed to read books file: %v", err)
	}
	var books []seedBook
	if err := json.Unmarshal(data, &books); err != nil {
		log.Fatalf("failed to parse books JSON: %v", err)
	}
```

The flags have sensible defaults for the service addresses (`localhost:50051` and `localhost:50052`), which match the ports exposed by Docker Compose. The `--books` flag defaults to the fixture file's path relative to the project root, so you can run the CLI from the repo root without specifying it.

### Login Flow

```go
	authConn, err := grpc.NewClient(*authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to auth service: %v", err)
	}
	defer authConn.Close()

	authClient := authv1.NewAuthServiceClient(authConn)
	loginResp, err := authClient.Login(context.Background(), &authv1.LoginRequest{
		Email: *email, Password: *password,
	})
	if err != nil {
		log.Fatalf("login failed: %v", err)
	}
	fmt.Println("Logged in successfully")
```

The seed CLI creates a gRPC connection to the auth service and calls `Login` to get a JWT. If login fails (wrong password, non-existent user, user is not admin), the process exits with a clear error.

### Metadata Injection and Book Creation

```go
	catalogConn, err := grpc.NewClient(*catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to catalog service: %v", err)
	}
	defer catalogConn.Close()

	catalogClient := catalogv1.NewCatalogServiceClient(catalogConn)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+loginResp.Token)
```

This is the key line: `metadata.AppendToOutgoingContext` attaches the JWT as gRPC metadata, equivalent to an HTTP `Authorization: Bearer <token>` header. Every subsequent `CreateBook` call uses this context, so the catalog service's JWT interceptor can authenticate and authorize the request.

### `AlreadyExists` Handling

```go
	created, skipped := 0, 0
	for _, b := range books {
		_, err := catalogClient.CreateBook(ctx, &catalogv1.CreateBookRequest{
			Title: b.Title, Author: b.Author, Isbn: b.ISBN,
			Genre: b.Genre, Description: b.Description,
			PublishedYear: b.PublishedYear, TotalCopies: b.TotalCopies,
		})
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code() == codes.AlreadyExists {
				fmt.Printf("  skipped (exists): %s\n", b.Title)
				skipped++
				continue
			}
			log.Fatalf("failed to create book %q: %v", b.Title, err)
		}
		fmt.Printf("  created: %s\n", b.Title)
		created++
	}
	fmt.Printf("\nDone: %d created, %d skipped\n", created, skipped)
```

The `AlreadyExists` handling makes the seed CLI idempotent -- running it twice will skip all 16 books on the second run instead of failing. This is the same pattern as the admin CLI's upsert logic, but here it leverages the catalog service's own duplicate ISBN check rather than implementing it in the CLI.

For any other error (e.g., validation failure, network issue), the CLI exits with `log.Fatalf`. This is appropriate for a seeding tool -- partial success is confusing, so fail fast and let the operator fix the issue.

---

## The Fixture File

The fixture file (`services/catalog/cmd/seed/books.json`) contains 16 books across six genres:

| Genre | Count | Examples |
|-------|-------|---------|
| Technology | 5 | *The Go Programming Language*, *Clean Code*, *The Pragmatic Programmer*, *Site Reliability Engineering*, *Designing Data-Intensive Applications* |
| Fiction | 4 | *To Kill a Mockingbird*, *1984*, *The Great Gatsby*, *Pride and Prejudice* |
| Science Fiction | 2 | *Dune*, *Neuromancer* |
| Science | 3 | *A Brief History of Time*, *Cosmos*, *The Selfish Gene* |
| History | 2 | *Sapiens*, *The Art of War* |

The ISBNs are real ISBN-13 values for these books. This matters for testing -- if you later add ISBN validation (checksum verification), the seed data will still work. Total copies range from 2 to 5, giving you enough variation to test reservation limits and "no copies available" scenarios.

---

## How It Works With and Without Kafka

The seed CLI does not interact with Kafka directly -- it calls `CreateBook` via gRPC, and the catalog service handles event publishing internally. The catalog service's `main.go` uses a `noopPublisher` when `KAFKA_BROKERS` is not set:

```go
// services/catalog/cmd/main.go

var publisher service.EventPublisher = &noopPublisher{}
if kafkaBrokers != "" {
	// ... create real Kafka publisher ...
}
```

This means:

- **Without Kafka:** Books are created in the catalog database. No events are published. The search service will not be updated (it relies on Kafka events to index books). This is fine for basic development and testing.
- **With Kafka:** Books are created and `book.created` events are published to the `catalog.books.changed` topic. The search service picks them up and indexes the books. This is the full production flow, covered in Chapter 7.

The seed CLI works identically in both cases -- it has no knowledge of whether Kafka is running.

---

## Usage

After creating an admin account (Section 6.1) and starting the stack:

```bash
go run ./services/catalog/cmd/seed \
  --email admin@library.local \
  --password admin123
```

Expected output:

```
Logged in successfully
  created: The Go Programming Language
  created: Designing Data-Intensive Applications
  created: Clean Code
  created: To Kill a Mockingbird
  created: 1984
  created: Dune
  created: A Brief History of Time
  created: Sapiens: A Brief History of Humankind
  created: The Pragmatic Programmer
  created: The Great Gatsby
  created: Cosmos
  created: The Art of War
  created: Neuromancer
  created: Site Reliability Engineering
  created: Pride and Prejudice
  created: The Selfish Gene

Done: 16 created, 0 skipped
```

Running it again:

```
Logged in successfully
  skipped (exists): The Go Programming Language
  skipped (exists): Designing Data-Intensive Applications
  ... (14 more skipped) ...

Done: 0 created, 16 skipped
```

If the services are running on non-default ports (e.g., when running outside Docker), use the `--auth-addr` and `--catalog-addr` flags:

```bash
go run ./services/catalog/cmd/seed \
  --email admin@library.local \
  --password admin123 \
  --auth-addr localhost:50051 \
  --catalog-addr localhost:50052
```

---

## Key Takeaways

- **Seed through the API, not the database.** This validates the entire stack -- auth, authorization, validation, and event publishing -- with every run.
- **Idempotent seeding via `AlreadyExists`.** The CLI can be run repeatedly without side effects, which is essential for CI pipelines and shared development databases.
- **Fixture files are test infrastructure.** Treat `books.json` as you would a test fixture: diverse, realistic, and checked into version control. When you add new features (e.g., ISBN validation), the fixture data should still work.
