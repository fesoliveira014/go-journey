# Chapter 11: Testing Strategies

Chapters 1 through 9 built a working five-service system: catalog, auth, reservation, search, and a gateway front-end. Along the way, you wrote unit tests for nearly every package — service-layer logic, handler routing, token validation, event publishing. Those tests run in milliseconds, require no external process, and catch a wide class of bugs.

They do not catch everything.

This chapter covers the failures unit tests miss, why they miss them, and what to do instead. By the end you will have a layered test suite that covers the system at three distinct levels of fidelity, with each layer doing exactly the job the one below it cannot.

---

## What unit tests cannot see

Consider the Catalog Service's `BookRepository`. In chapter 2 you defined an interface:

```go
type BookRepository interface {
    Create(ctx context.Context, book *domain.Book) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.Book, error)
    // ...
}
```

Your unit tests inject a hand-written mock that satisfies the interface, thoroughly exercising the service logic. But the mock has no SQL behind it. If the real GORM implementation contains a typo — say, `published_at` in the struct tag but `publish_date` in the migration — the mock will never surface that. Only a test that actually connects to a PostgreSQL instance and runs the query will catch it.

Now consider the gRPC layer: the `CatalogServer` is tested by calling its methods as plain Go functions. That is fast and convenient. But it means no gRPC frame ever travels over the wire, no interceptor chain runs, and no metadata is parsed. If the auth interceptor from chapter 4 is accidentally omitted from the server's option list, every unit test still passes. A test that dials a real (in-process) gRPC listener will fail immediately when it receives `codes.Unauthenticated`.

Finally, consider the Kafka consumer in the Reservation Service. You mocked `sarama.ConsumerGroup` and drove the `ConsumeClaim` loop directly. The message bytes were whatever your test constructed. If the Catalog Service's producer serializes events as Protobuf but the consumer's `Unmarshal` call expects JSON, the mismatch is invisible to both sides in isolation. Only a test that publishes a real message to a real Kafka broker — with the same serialization path the production code uses — can detect it.

These are not hypothetical edge cases. They are the three most common categories of integration failure in a Go microservices system:

1. **SQL/ORM mismatches** — wrong column names, missing migrations, incorrect transaction boundaries.
2. **gRPC wiring failures** — missing interceptors, wrong server options, incorrect service registration.
3. **Serialization mismatches** — producer and consumer using incompatible formats or schema versions.

---

## The testing pyramid

The testing pyramid[^1] describes a recommended distribution of tests across three levels. The shape reflects two properties that are in tension: **confidence** and **cost**.

```
        /\
       /  \
      / E2E\        fewest, slowest, highest confidence
     /------\
    /        \
   / Integr.  \     moderate count, moderate speed
  /------------\
 /              \
/   Unit Tests   \  most numerous, fastest, cheapest
------------------
```

**Unit tests** sit at the base. They are the most numerous because they are cheapest to write and run in milliseconds. Each test is narrow: one function, one method, one decision branch. They are the right tool for business logic: fee calculation, validation rules, state-machine transitions.

**Integration tests** occupy the middle. They are slower because they require real infrastructure: a PostgreSQL container, a Kafka broker, a running gRPC server. You write fewer of them and focus them on the seams between your code and external systems. They answer the question "does this SQL actually work?" and "does this consumer actually parse what the producer sends?"

**End-to-end (E2E) tests** sit at the top. In a microservices system, a full end-to-end test might start all five services and exercise a user-facing scenario through the gateway's HTTP API. They give the highest confidence but cost the most: tens of seconds to start containers, complex setup and teardown, and fragile dependencies on network timing. You keep them few and focused on critical user paths.

If you are coming from a Java/Spring background, think of unit tests as JUnit tests with Mockito, integration tests as `@SpringBootTest` with an in-memory or Testcontainers-backed data source, and e2e tests as REST Assured or Playwright suites that drive a fully deployed application. The taxonomy is identical; Go's tooling just looks different.

---

## The cost model

One practical way to think about the pyramid is in terms of wall-clock time per test run:

| Layer       | Typical duration      | Infrastructure needed         |
|-------------|-----------------------|-------------------------------|
| Unit        | 1–50 ms per test      | None                          |
| Integration | 5–30 s per suite      | Docker containers (per suite) |
| E2E         | 30–120 s per scenario | Multiple running services     |

Container startup is the dominant cost for integration tests. Testcontainers manages this well: it starts a container once per test suite (using `TestMain`), runs all tests in that suite against the same instance, then tears it down. A full integration suite for one service — covering every repository method against a real PostgreSQL instance — typically takes 10–20 seconds, most of which is the 5–8 seconds for the container to accept connections.

This cost model drives the build separation strategy. Unit tests run on every commit push, typically inside the CI build container itself (no Docker-in-Docker required). Integration and e2e tests run on pull requests or nightly, where the CI environment can provision containers. You enforce this separation in Go using build tags.

---

## Build tags for test separation

Go's build tag system lets you annotate a file so it is only compiled when a specific tag is provided. The convention for this project is:

```go
//go:build integration
```

A file with this tag at the top is excluded from `go test ./...`. To include it, you pass the tag explicitly:

```bash
# Unit tests only (default, fast)
go test ./...

# Integration tests included
go test -tags integration ./...
```

This is the Go equivalent of Gradle's `sourceSets` or Maven's `Failsafe` plugin separating unit and integration test lifecycles. The tag is a compiler directive, not a runtime flag — files without the tag are never compiled into the test binary at all.

The practical implication: your integration test files start with `//go:build integration` and may import packages like `testcontainers-go` and `github.com/docker/docker/client` that you do not want in the standard test binary.

---

## Tooling overview

Three tools do the heavy lifting in this chapter.

**testcontainers-go**[^2] is a Go library that starts Docker containers programmatically from within a test. You describe the image, exposed ports, environment variables, and wait strategy (e.g., "wait until port 5432 accepts TCP connections"), and the library handles the Docker API calls. It is the Go equivalent of the `testcontainers-java` library that Spring developers use with `@Testcontainers`.

```go
pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
    ContainerRequest: testcontainers.ContainerRequest{
        Image:        "postgres:16-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "catalog_test",
        },
        WaitingFor: wait.ForListeningPort("5432/tcp"),
    },
    Started: true,
})
```

**bufconn**[^3] is a package in the gRPC library that provides an in-process network connection. Instead of binding to a real TCP port, your gRPC server listens on a `bufconn.Listener`. Clients connect through a custom dialer that routes traffic through an in-memory buffer. The full gRPC stack — interceptors, codec, metadata — runs exactly as it does in production, but no operating-system network stack is involved. Tests run in milliseconds with full interceptor coverage.

**`//go:build integration`** is not a library but a compiler convention. It shapes how you organize files. All testcontainers-based tests and all multi-service tests carry this tag. Pure bufconn tests, which need no external infrastructure, can run without Docker and may or may not carry the tag depending on your project's conventions. This project uses the tag for anything that starts a container.

---

## Chapter roadmap

The remaining sections build the testing strategy layer by layer:

**11.1 — Unit Testing Patterns** revisits the mock-based approach from previous chapters with a critical eye. You will see when hand-written mocks are the right tool, when `gomock` or `testify/mock` is preferable, and how to structure table-driven tests for complex input spaces. This section also covers `t.Cleanup`, `t.Parallel`, and subtests — Go idioms that have no direct Kotlin/Java equivalent but pay dividends in test suite organization.

**11.2 — Integration Testing with Testcontainers** applies testcontainers-go to the Catalog and Reservation Service repositories. You will write a `TestMain` function that starts a PostgreSQL container, runs migrations using the same `golang-migrate` code the production service uses, and tears everything down after the suite. Every repository method gets a test against real SQL.

**11.3 — gRPC Testing with bufconn** wires up a full gRPC server — with the auth interceptor, the real service implementation, and the real repository (backed by the Testcontainers PostgreSQL from 11.2) — and dials it through a bufconn listener. You will test that unauthenticated calls are rejected, that valid JWTs are accepted, and that the server returns correct responses for normal operations.

**11.4 — Kafka Testing** covers the serialization seam between the Catalog Service's event producer and the Reservation and Search Service consumers. You will start a Kafka broker via Testcontainers, publish events through the production publisher code, consume them through the production consumer code, and assert that the full round-trip preserves all fields correctly.

**11.5 — Service-Level End-to-End Tests** composes the previous layers into a scenario-level test: a user reserves a book, the Catalog Service publishes a `BookReserved` event, the Reservation Service consumes it and updates its own database, and the search index is invalidated. The test starts all necessary containers, wires the services together, and drives the scenario through the gateway's HTTP API.

---

By the end of this chapter your test suite will be stratified. It will be fast where speed matters, thorough where thoroughness matters, and organized so CI can run the right level at the right time. The unit tests you already have are not wasted — they remain the fastest feedback loop for logic bugs. What you are building now is the infrastructure to catch the class of bugs they were never designed to find.

---

[^1]: The Test Pyramid — Martin Fowler: https://martinfowler.com/bliki/TestPyramid.html
[^2]: Testcontainers for Go: https://golang.testcontainers.org/
[^3]: gRPC bufconn package: https://pkg.go.dev/google.golang.org/grpc/test/bufconn
