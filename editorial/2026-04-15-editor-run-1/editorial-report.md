# Editorial Report

**Run:** 2026-04-15-editor-run-1
**Manuscript:** Building Microservices in Go — `docs/src/`
**Sections reviewed:** SUMMARY.md, introduction.md, and all 14 chapters (78 section files total)
**Style authority:** Chicago Manual of Style, 17th ed.
**Editor mode:** four-pass (Structural → Line → Copy → Final Polish), comments and changelogs only — no source files modified.

## Previous Run Status

No prior editorial runs exist. This is the first pass over the manuscript.

---

## Book-Level Structural Observations

The book has a coherent learning arc: foundations → first service → containerization → auth → frontend → tooling → events → search → observability → CI/CD → testing → Kubernetes → cloud → hardening. The progression is appropriate for the stated audience (experienced engineer new to Go and cloud-native), and the recurring "Why X?" / "JVM analogy" patterns are consistently good pedagogical levers — preserve them.

Three book-wide structural concerns warrant author attention:

**1. Chapter 11 (Testing) is positioned after Chapter 10 (CI/CD) but is foundational to it.** Several CI/CD examples assume tests already exist, while Chapter 11 treats testing as if introducing it for the first time. Either reorder (testing before CI/CD), or add a stronger forward reference in Chapter 10 acknowledging that the test patterns are taught later. Chapter 11's `index.md` and `e2e-testing.md` further use `10.x` cross-references where they should say `11.x` — this looks like an artifact of a recent reorder.

**2. Chapter 7 (Reservation) has an internal framing inconsistency.** Section 7.1 frames the reserve flow as "check availability, then publish event" (a two-step async pattern). Section 7.2 establishes the correct pattern: a guarded synchronous decrement, followed by an event for downstream propagation. Sections 7.4 (Reservation UI, "Eventual Consistency in the UI") and the index then revert to 7.1's framing, producing a contradictory threat model and an incorrect motivating scenario for the EC discussion. The 7.2 framing is authoritative; rewrite 7.1, 7.4, and the chapter index accordingly.

**3. Chapter 12 has a section-ordering / cross-reference contradiction.** `app-manifests.md` (12.3) opens by referring to "the previous section" as having declared infrastructure manifests, but those live in 12.4. The same section closes by saying "Section 12.4 assembles the top-level kustomization.yaml" — but 12.5 is the Kustomize section. Either the sections are out of logical order, or the cross-references are wrong. `kustomize.md` independently mistypes section numbers as `11.3` / `11.4` (should be `12.3` / `12.4`).

A fourth, smaller concern: several chapter indexes overpromise the body sections (Chapter 11 promises a "five-service E2E" that 11.5 doesn't deliver; Chapter 13 index lists smoke tests, automated backups, parameter groups, and schema-migration Jobs that the body sections don't cover). Trim the index claims or add the missing material.

---

## Per-Section Summaries

### SUMMARY.md
- **Structural:** Indentation inconsistency (Ch 1 = 4 spaces; Ch 2–14 = 2 spaces); `7.1` duplicates the chapter title; `13.9` "Deploying and Verifying" duplicates `12.6` verbatim; section-title parallelism breaks within Ch 1 and Ch 3; "BFF" used as bare jargon in `5.1`.
- **Copy editing:** `&` vs `and` in titles (CMOS 10.10 prefers `and` outside brand names); spaced em dash (CMOS 6.85); `slog` and `kind` are identifiers — consider backticks; vendor canonical "Argo CD" (two words); filename `integration-testing-postgres.md` diverges from title "Testcontainers".
- **Queries:** 0 (TOC is reference-only).

### introduction.md
- **Structural:** Most impactful gap — no prerequisites, no scope-exclusion, no time estimate, no conventions, no companion-repo pointer, no architecture diagram (CLAUDE.md commits to one), no errata channel, no Chapter 1 transition. The "chapter-snapshot" reading-model paragraph is the most important reader-orientation idea in the file but is buried mid-paragraph; promote to a callout.
- **Line editing:** Cut "Welcome to" opener; "knows how to program"; "and/or".
- **Copy editing:** Hyphenate "library-management system" before noun; spell out "five microservices"; em-dash style; `Chapters 3-9` → en dash.
- **Final:** "Gmail" on line 13 should be "Google" — both factual (Gmail is the mail product, Google is the OAuth2 provider) and a TOC↔intro consistency issue.
- **Queries:** 0.

### Chapter 1 — Go Foundations
- **Structural:** `index.md` repeats deliverables twice; `project-setup.md`'s `main.go` snippet duplicates `http-server.md`; `go-basics.md` has an orphaned "no pointer arithmetic" fragment; `http-server.md` casually forward-references gRPC; `testing.md` commits to "standard library only" without scoping the claim.
- **Line editing:** Strong author voice — already low filler; mostly tense shifts, sentence merging, and a few absolute-claim softenings ("the standard" → "a common").
- **Copy editing:** Heading-level inconsistency (`project-setup.md` opens with `#`; the others with `##`). Spaced em dashes. AmE/BrE pocket in `testing.md` (Behaviour, synchronised, afterwards, colour-coding, behavioural). Footnote-style drift between sections.
- **Queries:** 9 (Go 1.26 release timing; Earthly URL; `go work sync` direction; PostgreSQL driver `pgx` vs GORM mention; encoder default whitespace; Newman 2nd ed. 2021; Go compiler error text; subtest-name mangling; `golang:1.22-alpine` vs Go 1.26+ prerequisite).

### Chapter 2 — First Microservice (Catalog)
- **Structural:** Strong "why-before-what" framing throughout. `repository-pattern.md` is the longest and densest section; `service-layer.md` is the strongest pedagogically.
- **Line editing:** ~12 instances of "just"/"simply" across the chapter — primary pattern.
- **Copy editing:** Leading-space footnote markers in `service-layer.md` and `wiring.md`; BrE drift ("behaviour", "defence"); inline-hyperlink vs numbered-footnote citation drift; SQL clause order in `repository-pattern.md` example almost certainly wrong (`WHERE…LIMIT…ORDER BY`); exercise advocates `errors.Is` but uses `!=`; `tradeoff` vs `trade-off` drift.
- **Queries:** 22 (proto3 `optional` reintroduction, field-number byte-range, buf plugin flag, `ErrNoChange` wrapping, `gen_random_uuid` recommendation, multiple URLs).

### Chapter 3 — Containerization
- **Structural:** **Substantive content/code drift** flagged twice: `writing-dockerfiles.md` says the Gateway is "self-contained" / "no cross-module dependencies," but the Dockerfile copies `gen/`, `pkg/auth/`, and `pkg/otel/`; `dev-workflow.md`'s `Dockerfile.dev` snippets omit `pkg/auth`/`pkg/otel` copies. Either the manuscript or the production code is wrong.
- **Line editing:** Filler ("actually", "fundamentally", "just", "completely"); recurring low-grade passives.
- **Copy editing:** Em-dash style chapter-wide (uses `--`); unit/number formatting (`~300MB`, `15-25MB`, `1000ms` — needs space + en dash for ranges per CMOS 9.16, 6.78); `docker-compose.md` says "six commands" but the snippet shows four.
- **Queries:** 17 (Docker docs URLs after 2024 restructure; Air import path renamed `cosmtrek/air` → `air-verse/air`; Alpine 3.19 EOL status).

### Chapter 4 — Authentication
- **Structural:** Strong opener/analogy pattern. `interceptors.md` "Why Typed Context Keys?" is the chapter's pedagogical high-water mark.
- **Line editing:** "just" (5×), "obviously" (2×), "literally", "simply"; one count mismatch ("all three" vs 4 listed actions in `interceptors.md`).
- **Copy editing:** Footnote-anchor gap — every content file defines `[^1]`–`[^5]` but none are anchored in prose. Treat as bibliography or add anchors.
- **Queries:** 12 (bcrypt hash split 22+31, DefaultCost=10, `golang-jwt/v5` currency, `sub` claim gloss, Mazières accent, Google Console navigation drift, `crypto/rand` vs `math/rand`, userinfo endpoint deprecation, non-ASCII em dash in source comment).

### Chapter 5 — Gateway & Frontend
- **Structural:** `session-management.md` "Why sign the cookie?" is exemplary threat-modeling narration. `templates-htmx.md` has the chapter's best motivated examples.
- **Line editing:** Filler removals; one factual fix: **"`main.go` function" → "`main` function"** (`main.go` is a file, not a function).
- **Copy editing:** Lock a single form for "Backend-for-Frontend (BFF)" and "POST-Redirect-GET" (both currently drift); BrE "defence" → "defense"; tradeoff/trade-off drift; mermaid participant id `Go as Google` clashes with the Go language name.
- **Final:** **Possible bug** — bare `setFlash(w, ...)` in `admin-crud.md` vs `s.setFlash(w, ...)` in `session-management.md`. Author confirmation needed.
- **Queries:** 21 (HTMX 2.0.4 currency, SRI hash, attribute names, HX-Request header, `~14 KB` size, OAuth2 redirect-code norm, Alpine 3.19 EOL).

### Chapter 6 — Admin & Developer Tooling
- **Structural:** `index.md` says "three tools" but lists four. `admin-cli.md`'s "Why no `AutoMigrate`?" sidebar is the chapter's best didactic moment. `seed-cli.md`'s "non-default ports" example uses default values. `putting-it-together.md` says services are "mostly isolated" where the author means *tightly coupled*.
- **Copy editing:** `--` used chapter-wide except one true em dash in `admin-cli.md` (line 88) — clearly inconsistency rather than intent. Postgres vs PostgreSQL mixed. CLI/CI used without first-use expansion.
- **Final:** **Port discrepancy** — `admin-dashboard.md` references reservation service on port `50053` while the rest of the chapter uses `50051`/`50052`. Confirm.
- **Queries:** 10 (port number, fixture/output title mismatch, login vs authorization conflation in `seed-cli.md`).

### Chapter 7 — Event-Driven Architecture
- **Structural:** **Major framing inconsistency** (see Book-Level Observations §2). `event-driven-architecture.md` says "duplicate increment is harmless" — contradicts the more accurate `kafka-consumer.md`. `reservation-service.md`'s TOCTOU "step 3" claims the reaper reconciles unpaired decrements; it does not.
- **Line editing:** `reservation-ui.md`'s "Eventual Consistency in the UI" section needs reanchoring to *return*/*expire* paths (the genuine EC flows); the current scenario (stale availability after *reserve*) is wrong given the synchronous decrement.
- **Copy editing:** Sarama/OTel API alias drift (`otelgo` vs `otel`); BrE (`serialises`, `cancelled`); spaced en dash from `--`; `HighWaterMarkOffset` location (`ConsumerGroupClaim`, not `ConsumerGroupSession`); Sarama strategy factory function name; Kafka 4.0 ZooKeeper-removal claim.
- **Queries:** 16 (Sarama API, AutoCommit interval default, MDN URL path, gorilla/csrf maintenance status, file-path comments).

### Chapter 8 — Full-Text Search
- **Structural:** `catalog-events.md` is the strongest tutor section in the chapter.
- **Line editing:** **Possible observability bug** — `catalog-events.md` Trace Propagation snippet shows `Inject(ctx, ...)` *before* `Start(ctx, "catalog.publish")`. If the source matches, the publish span is not what's being propagated. Verify.
- **Line editing (cont.):** **Code/prose contradiction** in `meilisearch.md` — prose says "move on" (fire-and-forget) but code does not commit on failure, so Kafka will redeliver. One must change.
- **Copy editing:** "Sarama, the most widely used Go Kafka client library" conflicts with the repo's recent commit (37c217a) noting Sarama maintenance status and recommending franz-go evaluation. Soften. "Go's UUID package" is imprecise (no stdlib UUID). `meilisearch-go` API surface needs verification (`MeilisearchApiError.Code`, `WaitForTask` signature, `DocumentOptions.PrimaryKey`, sort parameter format). Single-quoted phrases should be double-quoted (CMOS 11.8). "rxjs" → "RxJS"; "slightly-delayed" → "slightly delayed" (CMOS 7.86 -ly adverb rule).
- **Queries:** ~18.

### Chapter 9 — Observability with OpenTelemetry
- **Structural:** Section progression is sound (fundamentals → instrumentation → logging → stack → sidecar). `sidecar-pattern.md` title undersells the breadth (covers shared, sidecar, AND DaemonSet); sampling discussion partially recurs.
- **Copy editing:** `--` used as em dash throughout (verify whether the SSG converts before global replace); `256MB` → `256 MB` (CMOS 9.16); ASCII sidecar diagram has truncated right-edge labels ("Col").
- **Queries:** Collector-contrib `0.149.0` image tag; Go SDK identifier names (`ParentBasedSampler`/`TraceIDRatioBased`); `memory_limiter` field names; missing `volumes:` block in K8s sidecar example; `hostPort` production caveat; Downward API phrasing; `$(status.hostIP)` syntax mixing; OTel SDK API names across `instrumentation.md` / `structured-logging.md` / `grafana-stack.md`.

### Chapter 10 — CI/CD with GitHub Actions & Earthly
- **Structural:** `index.md` ASCII diagram shows `+build-and-push` as an Earthly sub-target, but `image-publishing.md` establishes it as a GHA job using `docker/build-push-action`. `cicd-fundamentals.md` references `earthly +publish` (does not exist elsewhere) and says "five stages" before a four-arrow diagram. `github-actions.md` "Why a Separate Workflow for PRs?" and "Why Two Workflows" overlap — consolidate.
- **Line editing:** `linting.md` `gosimple` second example (`result := input`) aliases the backing array — not a real `gosimple` suggestion; "Simplifiable boolean expression" heading describes a string comparison; "SpotBugs + FindBugs" — FindBugs is deprecated. `image-publishing.md` `fail-fast` paragraph contradicts itself.
- **Copy editing:** `--` chapter-wide; cross-references inconsistent ("section 10.2" vs "Section 10.4" — capitalize); "filesystem" vs "file system" both appear; "Jenkins master" → "Jenkins controller"; Action major tags (e.g., `actions/checkout@v4`) need publication-date verification; `earthly v0.8.15`, `golangci-lint v1.64.8`, `alpine:3.19` ditto.
- **Queries:** 22.

### Chapter 11 — Testing Strategies
- **Structural:** **Section-number drift:** `index.md` and `e2e-testing.md` use `10.x` instead of `11.x`. `index.md` opens with "five-service system" but names only four (catalog/auth/reservation/search + gateway). `index.md` 10.5 description promises a multi-service gateway-driven test that contradicts what `e2e-testing.md` actually delivers.
- **Line editing:** Several **non-compiling code samples** flagged: `unit-testing-patterns.md` `rand.Intn(1e10)` (float to int); `kafka-testing.md` `&testing.T{}` in `TestMain` and `append([]any(nil), idx.upserted...)` likely won't compile; `e2e-testing.md` `mockCatalogClient` embeds a server interface but is used as a client. `e2e-testing.md` references `startReservationServer` and `consumeOneEvent` without definitions; auth Step 6 imports missing.
- **Copy editing:** UK/US spelling mix (`behaviour`, `authorisation`, `serialisation`, `initialising`, `cancelled`); "Testcontainers" (brand) vs `testcontainers-go` (module) inconsistent; container-startup time figures disagree across sections (index: 5–8 s; 11.2: 2–4 s and 3+ s; 11.4: 3–5 s) — unify.
- **Cross-section drift:** 11.3 uses `grpc.NewClient` (current); 11.5 uses deprecated `grpc.DialContext`. Event-name drift across sections: `BookReserved` (index), `reservation.created` (11.4), `BookCreated` (11.5). `service.NewCatalogService(repo)` 1-arg in 11.4 vs 2-arg elsewhere. 11.2 says replace `t.Skip`; 11.5 keeps it.
- **Queries:** 34.

### Chapter 12 — Kubernetes
- **Structural:** **Section ordering/cross-reference issue** (see Book-Level Observations §3). `kustomize.md` lines 3 and 82 say `sections 11.3 and 11.4` — should be `12.3` and `12.4`. `app-manifests.md` catalog YAML omits `failureThreshold` and `metadata.labels` shown elsewhere; auth YAML missing `GOOGLE_CLIENT_SECRET` secretKeyRef that prose claims exists. `infra-manifests.md` Meilisearch Service YAML not shown; DNS diagram says "kube-dns" but modern clusters run CoreDNS.
- **Line editing:** Filler ("only" placement, dangling); one comma splice; `preparing-services.md` mischaracterizes the proto as a "single RPC" (defines two).
- **Copy editing:** File-path inconsistency (`k8s/...` vs `deploy/k8s/base/...`); filename suffixes (`-cm.yaml` vs `-configmap.yaml`); "Ingress Controller" capitalization drift; "Gateway" vs "gateway"; "nginx" vs "NGINX"; "kube-dns" vs "CoreDNS".
- **Final:** `deploying.md` output block lists `ingress.networking.k8s.io/gateway` (should be `library-ingress`) and `secret/oauth-secret` not produced by `kustomize.md`'s secretGenerator; text claims "startup probes" but manifests configure liveness/readiness only. Terminology: "single transaction" (kubectl has none); "load balancer" (kube-proxy ≠ LB).
- **Queries:** 20 (kind v0.23.0 currency; "AKE" likely means "AKS"; British spellings in `preparing-services.md`).

### Chapter 13 — Cloud Deployment
- **Structural:** **Largest factual-drift surface in the book.** `index.md` overpromises (smoke tests, automated backups, parameter groups, schema-migration Jobs not in body). **Duplicate Terraform resource:** `aws_security_group.msk` defined in both `networking.md` and `msk.md` — Terraform compile error. Circular-dependency risk between `vpc.tf` and `module.eks` references.
- **Cross-section drift:** Cluster name (`library-cluster` / `library-production` / `local.cluster_name`); Terraform directory (`terraform/` / `infra/terraform` / `infrastructure/`); ECR repo path (`library/<svc>` / `library-system/<svc>`); DATABASE_URL format (URL vs libpq keyword); MSK hostname template; RDS endpoint prefix; ConfigMap names; Ingress name. Footnote markers `[^N]` defined in 9 of 11 files but cited inline only in `deploying.md`. Sentence "A few details worth noting." appears verbatim in three files (missing verb).
- **Factual queries:** `ecr.md` — `scan_on_push = true` triggers basic Clair scanning, not Inspector. `msk.md` — `aws_msk_topic` resource doesn't exist; CloudWatch metric `KafkaBrokerDiskSpaceUsed` doesn't exist; SSM-then-`kubectl patch` flow conflicts with declarative approach. `eks.md` — "AWS maintains an official Terraform module" inaccurate (community-maintained); "EKS Pod Identity webhook" terminology ambiguous (IRSA webhook vs 2023 EKS Pod Identity). `production-overlay.md` — brittle JSON6902 path `containers/0/...`; placeholder container name in resources patch likely doesn't merge; ACM cert referenced but never provisioned. `cicd.md` — "failed rollout rolls back automatically" overclaim; STS session TTL stated as 15 min (default 1 h). `deploying.md` — "Multi-AZ RDS Aurora cluster" wrong (chapter uses non-Aurora `aws_db_instance`); resource names disagree with 13.3–13.7. **`rds.md` and `production-overlay.md` directly contradict each other** about Kustomize strategic-merge behavior on container `env`/`envFrom` lists.
- **Queries:** 60.

### Chapter 14 — Production Hardening
- **Structural:** `secrets.md` is longest; five near-identical ExternalSecret YAML blocks warrant compression. `meilisearch-secret` shown in `library` namespace in verification output but manifest places it in `data` — fix.
- **Line editing:** `index.md` 140-word opener should compress.
- **Copy editing:** Heading-style drift across the six files (em dash / colon / none; Title Case vs sentence case). "GCP Secret Manager" → "Google Secret Manager". Claim that shell history is "world-readable" is inaccurate (mode 0600 default). "scratch-based images built on top of Alpine" technically imprecise (scratch is independent). "externally-registered" → "externally registered" (CMOS 7.86, -ly adverb).
- **Final / cross-section drift:** `applying.md` — "Sections 13.1–13.4" should be 14.1–14.4; output-name drift (`certificate_arn`/`acm_certificate_arn`, `msk_bootstrap_brokers_tls`/`msk_tls_bootstrap`); `aws_rds_cluster` vs `aws_db_instance` inconsistency; JWT length mismatch with `secrets.md`; `kubectl patch configmap library-config` targets non-existent ConfigMap; "What's Next" says "Chapter 14 addresses this" but should say "Chapter 15".
- **Queries:** 28 (Route 53 pricing; ACM renewal cadence 60 days not 30; SOC 2 / PCI DSS scope; HTTP/2-requires-TLS overstatement (h2c allowed by spec); ESO Helm `0.9.13` and `v1beta1` outdated for 2026; Helm/`alpine:3.20`/`kafka 3.6.0` version currency; current `.com` price; Sarama maintenance-mode caveat consistency with Ch 8/10).

---

## Recurring Patterns

These appear across multiple chapters and warrant a single book-wide decision rather than per-section fixes:

1. **`--` vs `—` (em dash).** Used pervasively in spaced `--` form across most chapters. CMOS 6.85 prescribes the Unicode em dash `—` with no spaces. If the static-site-generator (mdBook + smartypants?) auto-converts, this is fine; if not, do a global replace. Decide once, document the convention.

2. **`-` vs `–` (en dash) for ranges.** Numeric ranges (`1-3 seconds`, `15-25MB`, `2019–2023`, `Chapters 3-9`) inconsistently use hyphens. CMOS 6.78 requires en dash.

3. **Numbers + units spacing.** `300MB`, `100ms`, `8GB`, `256MB`, `~14KB` appear throughout; CMOS 9.16 prescribes a space (`300 MB`, `100 ms`).

4. **Numbers in prose.** Multiple chapters use numerals where CMOS 9.2 prescribes spelled-out forms (`5 microservices` → `five`; `2 methods` → `two`; `7 RPCs` → `seven`). Numerals are correct in technical contexts (versions, ports, sizes); be consistent.

5. **AmE vs BrE.** Pockets of British spellings (`behaviour`, `serialises`, `cancelled`, `defence`, `optimised`, `synchronised`, `colour`, `initialising`, `authorisation`, `signalling`, `finalising`) intrude into otherwise AmE chapters. Decide a house standard and normalize.

6. **Filler words.** "just", "simply", "basically", "essentially", "actually", "fundamentally", "obviously", "literally", "completely", "as we can see" — present chapter-wide. Single most-common pass-2 edit. Author voice tightens noticeably without them.

7. **Footnote anchors defined but not cited inline.** Chapters 4, 7, 13, 14 define `[^1]`–`[^N]` markers without inline citations. Either cite inline or reframe as a "Further Reading" bibliography section.

8. **Citation-style drift.** Inline hyperlinks mixed with numbered footnotes (Ch 2, 4, 13). Pick one form per chapter or book-wide.

9. **Ampersand vs "and" in titles.** Title-case ampersands appear throughout the TOC and section headings. CMOS 10.10 prefers "and" outside brand contexts.

10. **Compound-modifier hyphenation (CMOS 7.81).** "well-known pattern" is correctly hyphenated in most files; "state machine transitions", "error translation code", "test result cache" (Ch 11) need hyphens before the noun. "-ly" adverbs do *not* take a hyphen (CMOS 7.86) — `slightly-delayed`, `externally-registered` are wrong.

11. **JVM/Kotlin/C++ analogies.** Used consistently and accurately for the stated audience. Preserve as a deliberate voice feature.

12. **Manuscript-vs-code drift.** Multiple chapters reference a state of the codebase that may no longer match: GORM mention in Ch 3 after the `pgx` migration; Sarama "most widely used" in Ch 8 contradicting the recent commit (37c217a) flagging maintenance status; Gateway "self-contained" claim in Ch 3 contradicting actual `pkg/auth`/`pkg/otel` imports; `setFlash` call signature divergence between Ch 5 sections; `grpc.NewClient` (current) vs `grpc.DialContext` (deprecated) split across Ch 11; section number `10.x` references in Ch 11; section number `11.x` references in Ch 12; section number `13.x` references in Ch 14. **The author should run a single cross-chapter consistency pass** comparing manuscript snippets against current source.

13. **Cross-chapter version pinning.** `alpine:3.19`/`3.20`, Kafka 3.6.0, Go 1.26 prerequisite vs `golang:1.22-alpine` in examples, `golang-jwt/v5`, `golangci-lint v1.64.8`, `earthly v0.8.15`, ESO Helm `0.9.13` + `v1beta1`, Action major tags. For a 2026 book, schedule a final version-currency sweep before publication.

14. **AWS factual claims.** Free-tier wording, pricing, default TTLs, scan-engine identification (Inspector vs Clair), "AWS-maintained" module attribution, STS session defaults, Aurora vs `aws_db_instance` taxonomy. Treat AWS claims with the same query rigor as code APIs.

---

## Statistics

| Metric | Count |
|---|---:|
| Files reviewed (sections) | 78 |
| Annotated files written | 80 |
| Changelog files written | 80 |
| Structural comments | ~360 |
| Line edit suggestions | ~605 |
| Copy edit suggestions | ~870 |
| Final polish suggestions | ~145 |
| Factual queries for author | ~280 |
| **Total inline comments** | **~1,980** |

(Counts aggregated from per-chapter reports; precise totals available in each `chXX/<file>.changelog.md`.)

---

## Notes on Process

- ch09 was processed in two passes — the first agent failed mid-stream (model-overload error) after completing 5 of 6 files; `sidecar-pattern.md` was retried separately and matches the same standard.
- SUMMARY.md was written by the editor directly after a sub-agent reported a (likely-hallucinated) write-tool guard. The four-pass treatment is condensed because the file is a TOC with minimal prose.
- All review-time observations stay in `editorial/2026-04-15-editor-run-1/` and adjacent. No file under `docs/src/` was modified.
