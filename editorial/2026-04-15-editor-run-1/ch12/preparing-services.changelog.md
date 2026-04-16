# Changelog: preparing-services.md

## Pass 1: Structural / Developmental
- 5 comments. Themes:
  - Strong opening that motivates two concerns (graceful shutdown, health checks) and distinguishes Kubernetes's strictness from Compose's tolerance.
  - Sequence diagram + two-line "race" commentary is an effective pedagogical pairing.
  - Service-by-service walkthrough (catalog → auth → reservation → search → gateway) provides good repetition; each is small enough to not feel padded.
  - The "One note on variable naming" paragraph for reservation is a useful aside but breaks the rhythm — consider callout/aside formatting.
  - Three concrete failure modes (gRPC, Kafka, DB) anchor the abstract need for signal handling. Excellent.

## Pass 2: Line Editing
- **Line ~3:** split long sentence
  - Before: "Docker Compose was forgiving about both — containers could exit abruptly without consequence and Compose had no built-in mechanism to check whether a service was actually ready to handle traffic."
  - After: "Docker Compose was forgiving about both: containers could exit abruptly without consequence, and Compose had no built-in mechanism to check whether a service was ready to handle traffic."
  - Reason: 34 words in the original; "actually" is filler.
- **Line ~32:** restructure race condition sentence
  - Before: "endpoint removal and `SIGTERM` are issued concurrently — there is a race between traffic stopping and your process receiving the signal"
  - After: "Kubernetes issues endpoint removal and `SIGTERM` concurrently; a race exists between traffic stopping and your process receiving the signal"
  - Reason: active voice with Kubernetes as subject; removes "there is."
- **Line ~40:** "evaporate" is imprecise
  - Before: "PostgreSQL sees the connections evaporate and eventually times them out"
  - After: "PostgreSQL sees the connections terminate abruptly and eventually times them out"
  - Reason: "evaporate" is vivid but inaccurate — connections are force-closed, not vanished.
- **Line ~55:** remove redundant sentence
  - Before: "`GracefulStop` stops the server from accepting new connections and blocks until all active RPCs complete, then returns. The `Serve` call unblocks when `GracefulStop` is called."
  - After: "`GracefulStop` stops the server from accepting new connections and blocks until all active RPCs complete, which unblocks `Serve`."
  - Reason: fold the third sentence's content into the second.
- **Line ~221:** clarify subject
  - Before: "`http.ListenAndServe` returns `http.ErrServerClosed` when `Shutdown` is called"
  - After: "`server.ListenAndServe` (and the package-level `http.ListenAndServe`) returns `http.ErrServerClosed` when `Shutdown` is called"
  - Reason: the preceding diff replaces the package-level call with the method; be explicit.

## Pass 3: Copy Editing
- **Line ~32:** "30 seconds (by default)" — the mermaid diagram uses "30s" but prose uses "30 seconds." Prose style already correct; note inconsistency is in the diagram label, which is acceptable for compactness. No change.
- **Line ~215:** "err != http.ErrServerClosed" — modern idiomatic Go uses `errors.Is(err, http.ErrServerClosed)`. Query: the book's style elsewhere — if the project consistently uses `errors.Is`, standardize here. Functional equivalence for sentinel errors returned directly, but `errors.Is` is idiomatic for wrapped errors.
- **Line ~221:** "initialisation" — British spelling. Query: book style guide — British or American English? "Initialisation" and "signalling" (line ~322) are British forms; check for consistency across the book.
- **Line ~232:** "a single RPC" — the proto defines two RPCs (`Check` and `Watch`). Consider: "defines two RPCs" and note that only `Check` is used for Kubernetes probes.
- **Line ~268:** "accelerating endpoint removal on the next probe cycle" — a readiness probe failure alone does not remove endpoints instantly; `failureThreshold` (default 3) consecutive failures are required. Consider tightening: "causing the readiness probe to fail on subsequent cycles, which removes the endpoint after `failureThreshold` consecutive failures."
- **Lines ~277 & ~306:** "image: catalog:latest" / "image: gateway:latest" — mismatch with app-manifests.md (`library-system/catalog:latest`, `library-system/gateway:latest`). Normalize or add a note that these snippets are illustrative.
- **Lines ~331, 339, 355:** code fences missing language tags. Add ```bash.
- **Line ~322:** "signalling" — British spelling. See note on ~221.

## Pass 4: Final Polish
- **Line ~229:** verify gRPC probe native support claim: "Since Kubernetes 1.24" — accurate per https://kubernetes.io/blog/2022/05/13/grpc-probes-now-in-beta/ (beta in 1.24; GA in 1.27). Both phrasings commonly seen; the text is correct.
- **Line ~301:** verify: "`mux.HandleFunc("GET /healthz", srv.Health)`" — this is the Go 1.22+ routing syntax with method prefix in the pattern. Good; confirms the book assumes Go 1.22+.
- No typos, doubled words, or broken cross-references detected.
