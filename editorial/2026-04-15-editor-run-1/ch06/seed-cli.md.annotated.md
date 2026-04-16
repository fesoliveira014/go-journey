# 6.3 Catalog Seed CLI

## The Problem

<!-- [STRUCTURAL] Motivation parallels 6.1 ("The Problem" → "Why this design?" → walkthrough → usage → takeaways). Consistent chapter structure — reader-friendly. -->
<!-- [LINE EDIT] "Typing them in through the admin UI one at a time is slow and inconsistent -- every developer on the project would create different test data, making it impossible to reproduce bugs or write meaningful integration tests." — 36 words; reads well. Keep. -->
An empty catalog is useless for development. You need books to browse, reserve, and test against. Typing them in through the admin UI one at a time is slow and inconsistent -- every developer on the project would create different test data, making it impossible to reproduce bugs or write meaningful integration tests.
<!-- [COPY EDIT] CMOS 6.19 serial comma: "browse, reserve, and test against" — correct. -->

The solution is a **seed CLI**: a tool that loads a predefined set of books from a JSON fixture file and creates them through the catalog service's gRPC API.

---

## Why gRPC Seeding (Not Direct DB Insert)?

<!-- [STRUCTURAL] This contrastive heading immediately references 6.1's design — good continuity, and it reinforces the key lesson that the two CLIs embody *different* bootstrapping patterns for different reasons. -->

The admin CLI from Section 6.1 connects directly to the database. The seed CLI does not -- it authenticates as an admin and calls `CreateBook` through gRPC. This is a deliberate design choice:

- **Exercises the full stack.** The seed process goes through authentication (login via auth service), authorization (the JWT interceptor checks the admin role), validation (the catalog handler rejects missing titles or duplicate ISBNs), and event publishing (if Kafka is configured, `book.created` events are emitted).
<!-- [COPY EDIT] CMOS 6.19 serial comma: four-item parenthetical list; correct. -->
<!-- [LINE EDIT] 57-word bullet — long but structurally a list-within-a-list, which justifies the length. Keep. -->

- **Catches integration bugs.** If the auth service is misconfigured, or the catalog's JWT interceptor rejects the token, or a validation rule is wrong, the seed CLI will fail -- giving you early feedback.
<!-- [COPY EDIT] Three-clause "if … or … or" — serial comma not needed before each "or" in this construction; CMOS 6.19 applies to lists of items, not alternatives joined by "or". Current punctuation reads fine. -->

- **Mirrors real usage.** An admin adding books through the UI follows the same code path. The seed CLI is just an automated version of that workflow.
<!-- [LINE EDIT] "The seed CLI is just an automated version of that workflow." → "The seed CLI is an automated version of that workflow." Reason: "just" is on the cut-list. -->

<!-- [STRUCTURAL] Neat summary sentence that ties back to 6.1 without repeating its argument. -->
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

<!-- [STRUCTURAL] Note: the heading "Struct and Imports" — but the code block shows only the struct, no imports. Either show the imports too, or rename the heading to "The Seed Struct". -->
<!-- [LINE EDIT] "The `seedBook` struct mirrors the `CreateBookRequest` proto fields. It uses `json` struct tags for deserialization from the fixture file. Note the `int32` types for year and copies -- these match the protobuf field types, avoiding a conversion step." — Good. Keep. -->
The `seedBook` struct mirrors the `CreateBookRequest` proto fields. It uses `json` struct tags for deserialization from the fixture file. Note the `int32` types for year and copies -- these match the protobuf field types, avoiding a conversion step.
<!-- [COPY EDIT] "protobuf" — closed compound; industry standard. Correct. -->
<!-- [COPY EDIT] "int32" in prose — using backticks would make the type consistent with code; recommend `int32` → "`int32`". Minor. -->

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

<!-- [LINE EDIT] "The flags have sensible defaults for the service addresses (`localhost:50051` and `localhost:50052`), which match the ports exposed by Docker Compose." → Good. Keep. -->
The flags have sensible defaults for the service addresses (`localhost:50051` and `localhost:50052`), which match the ports exposed by Docker Compose. The `--books` flag defaults to the fixture file's path relative to the project root, so you can run the CLI from the repo root without specifying it.
<!-- [COPY EDIT] "the repo root" — "repo" is informal register; fine here given the chapter voice. Alternate: "the repository root". Style call. -->

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

<!-- [COPY EDIT] Please verify: `grpc.NewClient` is the current constructor as of google.golang.org/grpc v1.63+; earlier the function was `grpc.Dial`. Confirm the project's `go.mod` pins a version where `NewClient` is canonical (v1.63 or later). CMOS style permits this factual claim only with verification. -->

<!-- [LINE EDIT] "The seed CLI creates a gRPC connection to the auth service and calls `Login` to get a JWT. If login fails (wrong password, non-existent user, user is not admin), the process exits with a clear error." — Good. Keep. -->
The seed CLI creates a gRPC connection to the auth service and calls `Login` to get a JWT. If login fails (wrong password, non-existent user, user is not admin), the process exits with a clear error.
<!-- [COPY EDIT] CMOS 6.19 serial comma: "(wrong password, non-existent user, user is not admin)" — the items are clauses, not a parallel noun list; the Oxford comma is still correct here. -->
<!-- [STRUCTURAL] Minor technical nit: `Login` returns a JWT, but a non-admin user can log in successfully — the "user is not admin" failure mode actually surfaces at the `CreateBook` call, not at login. Consider: "If login fails (wrong password or non-existent user), or if the user is not admin, the first `CreateBook` will fail — in either case the process exits with a clear error." -->

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

<!-- [LINE EDIT] "This is the key line: `metadata.AppendToOutgoingContext` attaches the JWT as gRPC metadata, equivalent to an HTTP `Authorization: Bearer <token>` header." — Good. Keep. -->
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

<!-- [LINE EDIT] "The `AlreadyExists` handling makes the seed CLI idempotent -- running it twice will skip all 16 books on the second run instead of failing. This is the same pattern as the admin CLI's upsert logic, but here it leverages the catalog service's own duplicate ISBN check rather than implementing it in the CLI." — 51 words; borderline long but the pedagogical through-line (contrasting the two CLIs' idempotency strategies) justifies it. Keep. -->
The `AlreadyExists` handling makes the seed CLI idempotent -- running it twice will skip all 16 books on the second run instead of failing. This is the same pattern as the admin CLI's upsert logic, but here it leverages the catalog service's own duplicate ISBN check rather than implementing it in the CLI.
<!-- [COPY EDIT] CMOS 9.2: "16 books" — numerals for countable quantities; correct. Consistent with "16 times" in the index. -->

<!-- [LINE EDIT] "For any other error (e.g., validation failure, network issue), the CLI exits with `log.Fatalf`. This is appropriate for a seeding tool -- partial success is confusing, so fail fast and let the operator fix the issue." — Good. Keep. -->
For any other error (e.g., validation failure, network issue), the CLI exits with `log.Fatalf`. This is appropriate for a seeding tool -- partial success is confusing, so fail fast and let the operator fix the issue.
<!-- [COPY EDIT] CMOS 6.43: "e.g.," — comma after "e.g." correct in mid-sentence. -->
<!-- [COPY EDIT] "fail fast" — compound verb phrase; no hyphen as adverb-verb usage. When used as adjective ("a fail-fast policy"), hyphenate. Correct here. -->

---

## The Fixture File

The fixture file (`services/catalog/cmd/seed/books.json`) contains 16 books across five genres:
<!-- [COPY EDIT] "16 books across five genres" — CMOS 9.2: numerals for technical/countable quantities ("16 books"), spelled-out for small numbers ("five genres" is under 100 and non-technical). Mixed use is intentional and correct. -->

| Genre | Count | Examples |
|-------|-------|---------|
| Technology | 5 | *The Go Programming Language*, *Clean Code*, *The Pragmatic Programmer*, *Site Reliability Engineering*, *Designing Data-Intensive Applications* |
| Fiction | 4 | *To Kill a Mockingbird*, *1984*, *The Great Gatsby*, *Pride and Prejudice* |
| Science Fiction | 2 | *Dune*, *Neuromancer* |
| Science | 3 | *A Brief History of Time*, *Cosmos*, *The Selfish Gene* |
| History | 2 | *Sapiens*, *The Art of War* |
<!-- [FINAL] Title consistency: the table lists "*Sapiens*"; the expected-output block on line 196 uses "*Sapiens: A Brief History of Humankind*". Pick one canonical form and apply to both. -->
<!-- [COPY EDIT] CMOS 8.171 (italics for book titles) — applied correctly throughout the table. -->

<!-- [LINE EDIT] "The ISBNs are real ISBN-13 values for these books. This matters for testing -- if you later add ISBN validation (checksum verification), the seed data will still work. Total copies range from 2 to 5, giving you enough variation to test reservation limits and "no copies available" scenarios." — Good. Keep. -->
The ISBNs are real ISBN-13 values for these books. This matters for testing -- if you later add ISBN validation (checksum verification), the seed data will still work. Total copies range from 2 to 5, giving you enough variation to test reservation limits and "no copies available" scenarios.
<!-- [COPY EDIT] "ISBN-13" — the hyphenated form is the ISO-canonical name. Correct. -->
<!-- [COPY EDIT] CMOS 6.9: "no copies available" scenarios — double quotation marks; the period/comma-inside rule applies only inside the quoted string; here the quoted phrase is followed by "scenarios", so no trailing punctuation inside the quotes. Correct. -->

---

## How It Works With and Without Kafka
<!-- [COPY EDIT] CMOS 8.159 headline-style caps: prepositions of fewer than five letters are lowercased in headline style *except* when they form part of a phrasal verb. "With and Without" — both are prepositions; lowercase would be "With and without". In practice, consistent capitalization of both is also defensible (parallel structure). Chapter uses Title Case elsewhere; accept as-is. -->

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

<!-- [FINAL] Please verify the topic name: "catalog.books.changed". If Chapter 7 uses a different canonical name (e.g., `catalog.book.v1.changed`, or `books.events`), reconcile now. -->

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
<!-- [FINAL] The expected-output block lists "Sapiens: A Brief History of Humankind" — but the fixture-file table earlier shows it simply as "Sapiens". Either update the table to the full subtitle, or update the fixture to match the table. Single source of truth. -->

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
<!-- [STRUCTURAL] Minor: the "non-default ports" example then shows the default ports. A reader could reasonably be confused. Consider changing one of the flags to a non-default value (e.g., `--auth-addr localhost:60051`) so the example actually demonstrates the feature. -->

---

## Key Takeaways

<!-- [STRUCTURAL] Three strong takeaways, each distinct from the others and from 6.1's takeaways. Good reinforcement. -->

- **Seed through the API, not the database.** This validates the entire stack -- auth, authorization, validation, and event publishing -- with every run.
<!-- [COPY EDIT] CMOS 6.19 serial comma: four-item list; correct. -->
- **Idempotent seeding via `AlreadyExists`.** The CLI can be run repeatedly without side effects, which is essential for CI pipelines and shared development databases.
<!-- [COPY EDIT] "CI" — abbreviation for "continuous integration". First and only use in this chapter. CMOS 10.3: define on first use. Recommend "continuous integration (CI) pipelines". -->
- **Fixture files are test infrastructure.** Treat `books.json` as you would a test fixture: diverse, realistic, and checked into version control. When you add new features (e.g., ISBN validation), the fixture data should still work.
<!-- [COPY EDIT] CMOS 6.19: three-item list in the colon expansion "diverse, realistic, and checked into version control" — correct. -->
