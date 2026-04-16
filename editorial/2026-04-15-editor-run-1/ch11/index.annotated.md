# Chapter 11: Testing Strategies

<!-- [STRUCTURAL] Chapter opening establishes motivation through concrete examples from earlier chapters. Good: grounds the abstract "testing pyramid" discussion in code the reader already knows. Consider briefly previewing the chapter's terminal deliverable (a CI-runnable integration/e2e suite) in the opening paragraph to reinforce stakes. -->
<!-- [LINE EDIT] "Chapters 1 through 9 built a working five-service system: catalog, auth, reservation, search, and a gateway front-end." The list names only four of the five services. Either add the fifth (notifications? or whatever is the fifth) or revise count. -->
<!-- [COPY EDIT] Please verify: the claim that Chapters 1–9 cover the five services. Upstream chapter audit may be needed. -->
Chapters 1 through 9 built a working five-service system: catalog, auth, reservation, search, and a gateway front-end. Along the way you wrote unit tests for nearly every package — service-layer logic, handler routing, token validation, event publishing. Those tests run in milliseconds, require no external process, and catch a wide class of bugs.

They do not catch everything.

<!-- [LINE EDIT] "This chapter is about the failures that unit tests miss, why they miss them, and what you need to do instead." → "This chapter covers the failures unit tests miss, why they miss them, and what to do instead." Cuts filler and passive-leaning phrasing. -->
<!-- [COPY EDIT] "By the end you will have" — CMOS prefers a comma after introductory phrases longer than four words (6.36). Insert after "end". -->
This chapter is about the failures that unit tests miss, why they miss them, and what you need to do instead. By the end you will have a layered test suite that covers the system at three distinct levels of fidelity, with each layer doing exactly the job the one below it cannot.

---

## What unit tests cannot see

<!-- [STRUCTURAL] Strong opening section: three concrete scenarios map one-to-one onto the three integration categories enumerated at the end. This motivational structure is load-bearing for the rest of the chapter — keep it. -->
Consider the catalog service's `BookRepository`. In chapter 2 you defined an interface:

```go
type BookRepository interface {
    Create(ctx context.Context, book *domain.Book) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.Book, error)
    // ...
}
```

<!-- [COPY EDIT] "chapter 2" — CMOS 8.179 recommends lowercase when used generically, which matches current usage. No change; flagging for consistency check across all ch11 files. -->
<!-- [LINE EDIT] "Your unit tests inject a hand-written mock that satisfies the interface. The service logic is thoroughly exercised." → "Your unit tests inject a hand-written mock that satisfies the interface, thoroughly exercising the service logic." Combines two short sentences for flow. -->
<!-- [COPY EDIT] "published_at" vs "publish_date" — good concrete example; leave as is. Note em dashes used correctly without spaces. -->
Your unit tests inject a hand-written mock that satisfies the interface. The service logic is thoroughly exercised. But the mock has no SQL behind it. If the real GORM implementation contains a typo — say, `published_at` in the struct tag but `publish_date` in the migration — the mock will never surface that. Only a test that actually connects to a PostgreSQL instance and runs the query will catch it.

<!-- [LINE EDIT] "Now consider the gRPC layer. The `CatalogServer` is tested by calling its methods as plain Go functions." → "Now consider the gRPC layer: the `CatalogServer` is tested by calling its methods as plain Go functions." Colon tightens link. -->
<!-- [COPY EDIT] "chapter 4" — consistent with earlier lowercase usage. -->
Now consider the gRPC layer. The `CatalogServer` is tested by calling its methods as plain Go functions. That is fast and convenient. But it means no gRPC frame ever travels over the wire, no interceptor chain runs, and no metadata is parsed. If the auth interceptor from chapter 4 is accidentally omitted from the server's option list, every unit test still passes. A test that dials a real (in-process) gRPC listener will fail immediately when it receives `codes.Unauthenticated`.

<!-- [LINE EDIT] "Finally, consider the Kafka consumer in the reservation service. You mocked `sarama.ConsumerGroup` and drove the `ConsumeClaim` loop directly." — 28+14 words; fine as is but could read "Finally, the Kafka consumer: you mocked `sarama.ConsumerGroup` and drove `ConsumeClaim` directly." Optional tightening. -->
<!-- [COPY EDIT] "Protobuf" → "protobuf" is inconsistent with "Protobuf" later in the chapter. The Protocol Buffers brand uses initial cap (per Google docs). Normalize to "Protobuf" everywhere in ch11. -->
Finally, consider the Kafka consumer in the reservation service. You mocked `sarama.ConsumerGroup` and drove the `ConsumeClaim` loop directly. The message bytes were whatever your test constructed. If the catalog service's producer serializes events as Protobuf but the consumer's `Unmarshal` call expects JSON, the mismatch is invisible to both sides in isolation. Only a test that publishes a real message to a real Kafka broker — with the same serialization path the production code uses — can detect it.

<!-- [COPY EDIT] "These are not hypothetical edge cases." — fine. -->
These are not hypothetical edge cases. They are the three most common categories of integration failure in a Go microservices system:

1. **SQL/ORM mismatches** — wrong column names, missing migrations, incorrect transaction boundaries.
2. **gRPC wiring failures** — missing interceptors, wrong server options, incorrect service registration.
3. **Serialization mismatches** — producer and consumer using incompatible formats or schema versions.

---

## The testing pyramid

<!-- [STRUCTURAL] Pyramid section does double duty: introduces the model and sets up the cost/tooling narrative that follows. Good pacing. The ASCII art is fine but the labels could be tighter. -->
<!-- [LINE EDIT] "The testing pyramid[^1] is a model that describes the recommended distribution of tests across three levels." → "The testing pyramid[^1] describes a recommended distribution of tests across three levels." Cuts "is a model that". -->
The testing pyramid[^1] is a model that describes the recommended distribution of tests across three levels. The shape reflects two properties that are in tension: **confidence** and **cost**.

<!-- [COPY EDIT] The ASCII pyramid is acceptable but "Integr." abbreviation is awkward in a published book. Consider spelling "Integration" even if the triangle widens slightly. -->
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

<!-- [LINE EDIT] "Each test is narrow: one function, one method, one decision branch." — fine triplet. -->
<!-- [COPY EDIT] "state machine transitions" — compound adjective before noun? "state-machine transitions" per CMOS 7.81. -->
**Unit tests** sit at the base. They are the most numerous because they are cheapest to write and run in milliseconds. Each test is narrow: one function, one method, one decision branch. They are the right tool for business logic — fee calculation, validation rules, state machine transitions.

<!-- [LINE EDIT] 'They answer the question "does this SQL actually work?" and "does this consumer actually parse what the producer sends?".' → 'They answer questions like "does this SQL actually work?" and "does this consumer actually parse what the producer sends?"' The original's terminal period after the closing question mark is an error. -->
<!-- [COPY EDIT] CMOS 6.124: do not double a terminal mark; delete the final period after "sends?". -->
**Integration tests** occupy the middle. They are slower because they require real infrastructure: a PostgreSQL container, a Kafka broker, a running gRPC server. You write fewer of them, and you focus them on the seams between your code and external systems. They answer the question "does this SQL actually work?" and "does this consumer actually parse what the producer sends?".

<!-- [LINE EDIT] "a full end-to-end test might start all five services and exercise a user-facing scenario through the gateway's HTTP API" — acceptable; 22 words, one clause. -->
<!-- [COPY EDIT] "end-to-end tests" — on first technical use, introduce the abbreviation: "End-to-end (E2E) tests". Later uses ("e2e") should be standardized: E2E in prose, e2e acceptable in code/filenames. -->
**End-to-end tests** sit at the top. In a microservices system, a full end-to-end test might start all five services and exercise a user-facing scenario through the gateway's HTTP API. They give the highest confidence but cost the most: tens of seconds to start containers, complex setup and teardown, and fragile dependencies on network timing. You keep them few and focused on critical user paths.

<!-- [LINE EDIT] "If you are coming from a Java/Spring background, think of unit tests as JUnit tests with Mockito, integration tests as `@SpringBootTest` with an in-memory or Testcontainers-backed datasource, and e2e tests as RestAssured or Playwright suites that drive a fully-deployed application." 50 words; consider splitting: "If you are coming from Java/Spring: think unit = JUnit + Mockito, integration = `@SpringBootTest` with an in-memory or Testcontainers datasource, e2e = RestAssured or Playwright driving a deployed application." -->
<!-- [COPY EDIT] "fully-deployed" — hyphen correct before noun (7.81). "Testcontainers-backed" — correct. "Java/Spring" — slash acceptable as "or" shorthand. -->
If you are coming from a Java/Spring background, think of unit tests as JUnit tests with Mockito, integration tests as `@SpringBootTest` with an in-memory or Testcontainers-backed datasource, and e2e tests as RestAssured or Playwright suites that drive a fully-deployed application. The taxonomy is identical; Go's tooling just looks different.

---

## The cost model

<!-- [STRUCTURAL] "The cost model" as a heading is oversold for a section with one table and three paragraphs; consider merging with "The testing pyramid" or retitling "Cost per layer". -->
One practical way to think about the pyramid is in terms of wall-clock time per test run:

<!-- [COPY EDIT] Table: en dash in ranges (1–50, 5–30, 30–120). Already correct. Good. -->
<!-- [COPY EDIT] "Docker containers (per suite)" vs "Multiple running services" — parallel phrasing would read better. Consider "Docker containers (one or more per suite)" vs "Multiple services + containers". -->
| Layer       | Typical duration      | Infrastructure needed         |
|-------------|-----------------------|-------------------------------|
| Unit        | 1–50 ms per test      | None                          |
| Integration | 5–30 s per suite      | Docker containers (per suite) |
| E2E         | 30–120 s per scenario | Multiple running services     |

<!-- [LINE EDIT] "Container startup is the dominant cost for integration tests. Testcontainers-go manages this well: it starts a container once per test suite (using `TestMain`), runs all tests in that suite against the same instance, then tears it down." — reads fine. -->
<!-- [COPY EDIT] "Testcontainers-go" — note product name formatting. Testcontainers (capital T) is the brand; "testcontainers-go" is the Go module/package name. Use "Testcontainers" in prose and `testcontainers-go` in code/monospace. Apply throughout. -->
<!-- [COPY EDIT] "5–8 seconds" — numerals with unit abbreviation is correct for technical measurement; "5 to 8 seconds" also acceptable. Be consistent — later sections use both. Pick one: recommend numerals + "s" for all durations in tables, "seconds" in prose. -->
Container startup is the dominant cost for integration tests. Testcontainers-go manages this well: it starts a container once per test suite (using `TestMain`), runs all tests in that suite against the same instance, then tears it down. A full integration suite for one service — covering every repository method against a real PostgreSQL instance — typically takes 10–20 seconds, most of which is the 5–8 seconds for the container to accept connections.

<!-- [LINE EDIT] "This cost model drives the build separation strategy." — good topic sentence. -->
<!-- [COPY EDIT] "pull requests or nightly" — terse; consider "on pull requests or in nightly runs". -->
This cost model drives the build separation strategy. Unit tests run on every commit push, typically inside the CI build container itself (no Docker-in-Docker required). Integration and e2e tests run on pull requests or nightly, where the CI environment can provision containers. You enforce this separation in Go using build tags.

---

## Build tags for test separation

<!-- [STRUCTURAL] This section introduces build tags but repeats material covered in 11.2 and 11.4. Consider leaving only the principle here and pushing the examples to 11.2 where they are used. Alternatively, keep as-is as a forward pointer; current placement is defensible. -->
Go's build tag system lets you annotate a file so it is only compiled when a specific tag is provided. The convention for this project is:

```go
//go:build integration
```

<!-- [LINE EDIT] "A file with this tag at the top is excluded from `go test ./...`. To include it, you pass the tag explicitly:" — fine. -->
A file with this tag at the top is excluded from `go test ./...`. To include it, you pass the tag explicitly:

```bash
# Unit tests only (default, fast)
go test ./...

# Integration tests included
go test -tags integration ./...
```

<!-- [LINE EDIT] "This is the Go equivalent of Gradle's `sourceSets` or Maven's `failsafe` plugin separating unit and integration test lifecycles." → "This is the Go equivalent of Gradle's `sourceSets` or Maven's Failsafe plugin, which separate unit and integration test lifecycles." Gradle and Maven plugin capitalization. -->
<!-- [COPY EDIT] "Failsafe" is the Maven plugin's canonical name (capitalized); "failsafe" lowercase is the artifact id. In prose, capitalize. -->
This is the Go equivalent of Gradle's `sourceSets` or Maven's `failsafe` plugin separating unit and integration test lifecycles. The tag is a compiler directive, not a runtime flag — files without the tag are never compiled into the test binary at all.

<!-- [LINE EDIT] "The practical implication: your integration test files start with `//go:build integration` and may import packages like `testcontainers-go` and `github.com/docker/docker/client` that you do not want in the standard test binary." 37 words; fine. -->
<!-- [COPY EDIT] Please verify: whether `github.com/docker/docker/client` is a direct import of testcontainers-go or only a transitive dependency. Readers may otherwise assume they import it directly. -->
The practical implication: your integration test files start with `//go:build integration` and may import packages like `testcontainers-go` and `github.com/docker/docker/client` that you do not want in the standard test binary.

---

## Tooling overview

<!-- [STRUCTURAL] Good idea to preview the tools but lists three items, two of which are libraries and one a compiler convention — the mixed taxonomy is slightly jarring. Consider subheads: "Libraries (testcontainers-go, bufconn)" and "Build-tag convention". -->
Three tools do the heavy lifting in this chapter.

<!-- [LINE EDIT] "testcontainers-go[^2] is a Go library that starts Docker containers programmatically from within a test." — fine. -->
<!-- [COPY EDIT] "testcontainers-java" should be `testcontainers-java` in monospace or "Testcontainers for Java"; pick one. CMOS 7.77 for product/library naming consistency. -->
<!-- [COPY EDIT] Please verify: `testcontainers.GenericContainer` API signature. The current API is `testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{...})` but the Postgres module uses `postgres.Run(...)` which is different. Confirm the generic example compiles against current testcontainers-go. -->
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

<!-- [LINE EDIT] "bufconn[^3] is a package in the gRPC library that provides an in-process network connection." — fine. -->
<!-- [COPY EDIT] "net.Listener" and "net.Conn" not referenced here but the copy says "in-process network connection". Acceptable. "real TCP port" — fine. -->
<!-- [COPY EDIT] "operating system network stack" → "operating-system network stack" — compound adjective modifying "network stack" (7.81). -->
**bufconn**[^3] is a package in the gRPC library that provides an in-process network connection. Instead of binding to a real TCP port, your gRPC server listens on a `bufconn.Listener`. Clients connect through a custom dialer that routes traffic through an in-memory buffer. The full gRPC stack — interceptors, codec, metadata — runs exactly as it does in production, but no operating system network stack is involved. Tests run in milliseconds with full interceptor coverage.

<!-- [LINE EDIT] "`//go:build integration` is not a library but a compiler convention." — grammatical but opens with monospace; consider "The `//go:build integration` directive is not a library but a compiler convention." -->
<!-- [COPY EDIT] "testcontainers-based" — compound hyphenated correctly. "bufconn tests" — lowercase, matches the package name. -->
**`//go:build integration`** is not a library but a compiler convention. It is worth calling out explicitly because it shapes how you organize files. All testcontainers-based tests and all multi-service tests carry this tag. Pure bufconn tests, which need no external infrastructure, can run without Docker and may or may not carry the tag depending on your project's conventions. This project uses the tag for anything that starts a container.

---

## Chapter roadmap

<!-- [STRUCTURAL] Major issue: the roadmap uses section numbers 10.1 through 10.5. This appears to be a remnant from an earlier chapter numbering. Chapter is now 11; numbers must read 11.1 through 11.5. Fix all five. -->
<!-- [FINAL] Section-number typos: 10.1→11.1, 10.2→11.2, 10.3→11.3, 10.4→11.4, 10.5→11.5. Also in the body: "from 10.2" → "from 11.2". -->
The remaining sections build the testing strategy layer by layer:

**10.1 — Unit Testing Patterns** revisits the mock-based approach from previous chapters with a critical eye. You will see when hand-written mocks are the right tool, when `gomock` or `testify/mock` is preferable, and how to structure table-driven tests for complex input spaces. This section also covers `t.Cleanup`, `t.Parallel`, and subtests — Go idioms that have no direct Kotlin/Java equivalent but pay dividends in test suite organization.

<!-- [COPY EDIT] "Testcontainers-go" capitalization — per CMOS and vendor style, use "Testcontainers" in prose. -->
**10.2 — Integration Testing with Testcontainers** applies testcontainers-go to the catalog and reservation service repositories. You will write a `TestMain` function that starts a PostgreSQL container, runs migrations using the same `golang-migrate` code the production service uses, and tears everything down after the suite. Every repository method gets a test against real SQL.

<!-- [LINE EDIT] "wires up a full gRPC server — with the auth interceptor, the real service implementation, and the real repository (backed by the Testcontainers PostgreSQL from 10.2) — and dials it through a bufconn listener." 36 words; dense but acceptable. -->
<!-- [COPY EDIT] "bufconn listener" — consistent lower case for the package name. -->
**10.3 — gRPC Testing with bufconn** wires up a full gRPC server — with the auth interceptor, the real service implementation, and the real repository (backed by the Testcontainers PostgreSQL from 10.2) — and dials it through a bufconn listener. You will test that unauthenticated calls are rejected, that valid JWTs are accepted, and that the server returns correct responses for normal operations.

<!-- [COPY EDIT] "catalog service's event producer" possessive chain; acceptable. -->
**10.4 — Kafka Testing** covers the serialization seam between the catalog service's event producer and the reservation and search service consumers. You will start a Kafka broker via Testcontainers, publish events through the production publisher code, consume them through the production consumer code, and assert that the full round-trip preserves all fields correctly.

<!-- [LINE EDIT] "composes the previous layers into a scenario-level test" — good summary. "a user reserves a book, the catalog service publishes a `BookReserved` event, the reservation service consumes it and updates its own database, and the search index is invalidated" — 33 words, parallel; fine. -->
<!-- [COPY EDIT] "BookReserved" vs "reservation.created" used later. Fact-check: the actual event name used in earlier chapters. Unified naming would help. Please verify: current event schema name. -->
**10.5 — Service-Level End-to-End Tests** composes the previous layers into a scenario-level test: a user reserves a book, the catalog service publishes a `BookReserved` event, the reservation service consumes it and updates its own database, and the search index is invalidated. The test starts all necessary containers, wires the services together, and drives the scenario through the gateway's HTTP API.

<!-- [STRUCTURAL] 11.5 as described here is a full multi-service test driven through the gateway, but 11.5 as written is a *service-level* e2e test that explicitly excludes multi-service flows. This is a direct contradiction. Rewrite this paragraph to match the actual content of 11.5. -->

---

<!-- [LINE EDIT] "By the end of this chapter, your test suite will be stratified, fast where it needs to be fast, thorough where thoroughness matters, and organized so that CI can run the right level of testing at the right time." 41 words; split. Suggest: "By the end of this chapter your test suite will be stratified. It will be fast where speed matters, thorough where thoroughness matters, and organized so CI can run the right level at the right time." -->
<!-- [COPY EDIT] Comma after "chapter" — CMOS 6.36 recommends it for introductory prepositional phrases of 4+ words. -->
By the end of this chapter, your test suite will be stratified, fast where it needs to be fast, thorough where thoroughness matters, and organized so that CI can run the right level of testing at the right time. The unit tests you already have are not wasted — they remain the fastest feedback loop for logic bugs. What you are building now is the infrastructure to catch the class of bugs they were never designed to find.

---

<!-- [COPY EDIT] Footnote URLs: confirm all still resolve. "https://martinfowler.com/bliki/TestPyramid.html" and "https://golang.testcontainers.org/" were valid as of 2025; re-verify. -->
[^1]: The Test Pyramid — Martin Fowler: https://martinfowler.com/bliki/TestPyramid.html
[^2]: Testcontainers for Go: https://golang.testcontainers.org/
[^3]: gRPC bufconn package: https://pkg.go.dev/google.golang.org/grpc/test/bufconn
