# Building Microservices in Go

<!-- [STRUCTURAL] The book lacks a stated prerequisite list. A reader new to Go won't know what's expected. Add a short "Prerequisites" subsection: programming experience assumed, OS expectations (Linux/macOS/WSL2), required local tools (Go 1.22+, Docker, kubectl, Terraform, an editor with gopls), AWS account for Ch 13-14, and rough disk/RAM expectations for the local stack (Postgres + Kafka + Meilisearch + kind cluster is non-trivial). -->
<!-- [STRUCTURAL] No "What This Is Not" section. Setting expectations on what's deliberately out of scope (e.g., advanced distributed-systems theory, multi-region failover, fine-grained RBAC, frontend frameworks beyond HTMX, mobile clients) prevents misaligned reader expectations and reduces drop-off. -->
<!-- [STRUCTURAL] No estimate of time investment. Tutors should signal scope: e.g., "Expect 40-60 hours of focused work to complete the full guide; each chapter is ~2-4 hours of reading plus exercises." Readers planning study time will appreciate this. -->
<!-- [STRUCTURAL] No conventions section. Define the typographic conventions used: code blocks, file paths, terminal commands ($/# prompts), inline code, callouts (Note/Warning/Tip), and how the book represents diff/added/removed code. -->
<!-- [STRUCTURAL] No mention of the companion repository (assumed to exist). Where is the source code? How is it tagged per chapter? Can readers `git checkout chN-end` to see the state at any chapter boundary? Critical for the "snapshot" model described in "How to Use This Guide". -->
<!-- [STRUCTURAL] No "Errata / Feedback" channel mentioned (issue tracker, email, etc.). Standard for technical books. -->
<!-- [STRUCTURAL] The bullet list in the opening paragraph mixes language ("Go"), pattern ("Microservices"), and infrastructure layers. Consider grouping under sub-headings (Application stack / Infrastructure / Operations) so the reader sees the architecture, not a flat tool list. -->

Welcome to this hands-on guide to building a complete microservices application in Go. By the end of this tutorial, you will have built a library management system covering:
<!-- [LINE EDIT] "Welcome to this hands-on guide to building a complete microservices application in Go." → "This hands-on guide walks you through building a complete microservices application in Go." (Cuts the empty welcome opener; leads with the verb.) -->
<!-- [LINE EDIT] "By the end of this tutorial, you will have built a library management system covering:" → "By the end, you will have built a library-management system that covers:" (Removes "of this tutorial" — redundant; "covering" dangles, attach with "that covers".) -->
<!-- [COPY EDIT] CMOS 7.89: "library-management system" hyphenates the compound modifier before the noun. -->

- **Go** — the language, project structure, testing, and idioms
- **Microservices** — service decomposition, gRPC, event-driven architecture with Kafka
- **Databases** — PostgreSQL with migrations and the repository pattern
- **Containers** — Docker, multi-stage builds, Docker Compose
- **Orchestration** — Kubernetes (kind locally, EKS in production)
- **Infrastructure as Code** — Terraform for AWS (VPC, EKS, RDS)
- **Observability** — OpenTelemetry, Tempo, Prometheus, Grafana, Loki
- **CI/CD** — GitHub Actions and Earthly
- **Authentication** — JWT, bcrypt, OAuth2 with Gmail
<!-- [COPY EDIT] CMOS 6.85: em dashes — these are spaced em dashes (typographic variant). CMOS strict house style uses unspaced em dashes ("**Go**—the language..."). Pick one and apply consistently across the book and TOC. The TOC uses spaced em dashes, so this is internally consistent — flag only the project-level decision. -->
<!-- [COPY EDIT] CMOS 6.19 serial comma: present in every list item — correct. -->
<!-- [COPY EDIT] "OAuth2 with Gmail" — Gmail is the mail product; the OAuth2 identity provider is "Google". The TOC (4.3) correctly says "OAuth2 with Google". Reconcile: change to "OAuth2 with Google" here, and in CLAUDE.md. -->
<!-- [COPY EDIT] "kind" should arguably be `kind` (backticks) to signal it's a tool/code identifier, since it's a common English word. Same logic for `slog` later in the book. -->
<!-- [LINE EDIT] "Microservices — service decomposition, gRPC, event-driven architecture with Kafka": consider "Microservices — decomposition, gRPC, and event-driven architecture with Kafka" (drops the redundant "service" before "decomposition"). -->
<!-- [LINE EDIT] "Containers — Docker, multi-stage builds, Docker Compose" → "Containers — Docker, multi-stage builds, and Docker Compose" (CMOS 6.19 serial comma + "and" before final item, parallel with other bullets). Several bullets omit "and" before the last comma item; standardize. -->

## Who This Is For

<!-- [STRUCTURAL] Section is one paragraph long. Consider expanding into: (1) the ideal reader profile, (2) prerequisite knowledge expected, (3) what readers will *not* learn here. Currently does both (1) and a hint of (2) but leaves prerequisites implicit. -->

You are an experienced software engineer who knows how to program but is new to Go and/or cloud-native tooling. The guide assumes strong programming fundamentals but explains Go-specific concepts, infrastructure patterns, and architectural decisions from scratch.
<!-- [LINE EDIT] "You are an experienced software engineer who knows how to program but is new to Go and/or cloud-native tooling." → "You are an experienced software engineer who is new to Go, cloud-native tooling, or both." (Cuts "knows how to program" — redundant after "experienced software engineer"; replaces "and/or" which CMOS 5.250 discourages.) -->
<!-- [COPY EDIT] CMOS 5.250: avoid "and/or" — use "X, Y, or both". -->
<!-- [LINE EDIT] "The guide assumes strong programming fundamentals but explains Go-specific concepts, infrastructure patterns, and architectural decisions from scratch." → "The guide assumes strong programming fundamentals and explains Go-specific concepts, infrastructure patterns, and architectural decisions from scratch." ("but" implies contradiction; "and" is the actual relationship — the assumption and the explanations coexist.) -->
<!-- [COPY EDIT] CMOS 6.19: serial comma present — correct. -->

## The Project

We are building a **library management system** where:
<!-- [LINE EDIT] "We are building a **library management system** where:" → "We will build a **library-management system** where:" (Future tense matches "By the end you will have built" above; present continuous is awkward in an introduction.) -->
<!-- [COPY EDIT] CMOS 7.89: "library-management system" — compound modifier before noun, hyphenate. (Same as the bullet list above.) -->

- Admins manage the book catalog (CRUD operations)
- Users browse, search, reserve, and return books
- Authentication supports email/password and Google OAuth2
<!-- [COPY EDIT] "email/password" — slash is acceptable per CMOS 6.106 for established pairings; alternatively spell out "email and password". -->
<!-- [COPY EDIT] "Google OAuth2" — consistent with TOC; reinforces the case for changing the bullet list above to "Google" not "Gmail". -->
<!-- [LINE EDIT] Bullet 1: "Admins manage the book catalog (CRUD operations)" → "Admins manage the book catalog (create, read, update, delete operations)" on first appearance — CRUD is jargon and the book hasn't yet defined it. Or: leave as-is and add a glossary. -->

The system is decomposed into 5 microservices: Gateway, Auth, Catalog, Reservation, and Search.
<!-- [COPY EDIT] CMOS 9.2: spell out whole numbers under 100 in narrative prose. "5 microservices" → "five microservices". -->
<!-- [STRUCTURAL] This is a one-line statement of the architecture but offers no diagram. An ASCII or image diagram showing the five services and their interaction (gRPC arrows, Kafka topics, Postgres connections) would dramatically help orientation. The CLAUDE.md commits to "architecture diagrams" — deliver one here. -->
<!-- [LINE EDIT] "The system is decomposed into 5 microservices" → "The system decomposes into five microservices" (active voice; CMOS 9.2 number rule). -->

## How to Use This Guide

Each chapter builds on the previous one. Follow them in order. **The code snippets in each chapter show the codebase as it exists at that point in the journey.** Later chapters modify and extend these files -- so if you compare a snippet from Chapter 2 to the final repository, you will see additions from Chapters 3-9 (Kafka integration, OpenTelemetry, structured logging, etc.). This is intentional: each chapter teaches one layer at a time.
<!-- [STRUCTURAL] This paragraph carries the most important reader-orientation insight in the entire introduction (the chapter-snapshot model), but it's buried mid-paragraph. Promote to a callout/note block (e.g., "> **Note: Snapshot model.** The code in each chapter..."). mdBook supports admonitions via the `mdbook-admonish` preprocessor. -->
<!-- [STRUCTURAL] Add a sentence on what the reader should DO with each chapter: type the code? clone the companion repo? checkout per-chapter tags? Without this, "follow them in order" is incomplete instruction. -->
<!-- [LINE EDIT] "Each chapter builds on the previous one. Follow them in order." → "Each chapter builds on the previous one, so follow them in order." (Combines two short sentences; smoother rhythm.) -->
<!-- [LINE EDIT] "Later chapters modify and extend these files -- so if you compare a snippet from Chapter 2 to the final repository, you will see additions from Chapters 3-9" → "Later chapters modify and extend these files, so a snippet from Chapter 2 will look different in the final repository — you'll see additions from Chapters 3-9" (Active framing; the reader is the implicit comparer.) -->
<!-- [COPY EDIT] "--" (double hyphen) appears as a typographic em dash. CMOS 6.85: use a true em dash (—). Replace "--" with "—". -->
<!-- [COPY EDIT] "Chapters 3-9": CMOS 6.78 — use en dash (–) for ranges, not hyphen (-). "Chapters 3–9". -->
<!-- [COPY EDIT] "(Kafka integration, OpenTelemetry, structured logging, etc.)" — CMOS 5.250: "etc." is acceptable but consider "and so on" or list a concrete final item. CMOS 6.20: "etc." is preceded by a comma in a list. Correct here. -->
<!-- [COPY EDIT] "Chapter 2" capitalized when followed by a number — correct (CMOS 8.179). -->

Every chapter includes:

- **Theory** — why we are making each decision
- **Implementation** — complete, runnable code
- **Exercises** — practice problems to test your understanding
- **References** — links to official docs and further reading
<!-- [LINE EDIT] "Theory — why we are making each decision" → "Theory — why each decision matters" (more concise; removes weak "we are making"). -->
<!-- [LINE EDIT] "Implementation — complete, runnable code" — fine as-is. -->
<!-- [LINE EDIT] "Exercises — practice problems to test your understanding" → "Exercises — problems that reinforce the chapter" (less self-referential). -->
<!-- [LINE EDIT] "References — links to official docs and further reading" → "References — links to official documentation and further reading" (CMOS 5.220: avoid abbreviating "documentation" in formal prose). -->
<!-- [COPY EDIT] Em dashes consistently spaced throughout the bullet list; matches the bullet list at the top. Internal consistency is good — only the project-level spaced-vs-unspaced decision remains. -->
<!-- [STRUCTURAL] No closing transition. The introduction ends abruptly on the bullet list. Add one sentence directing the reader to Chapter 1: "Ready? Turn to Chapter 1 to set up your Go environment and project skeleton." -->
<!-- [FINAL] Line 31: "--" should be "—" (em dash). -->
<!-- [FINAL] Line 31: "Chapters 3-9" should be "Chapters 3–9" (en dash for range). -->
<!-- [FINAL] Line 27: "5 microservices" → "five microservices" per CMOS 9.2. -->
<!-- [FINAL] Line 13: "Gmail" should be "Google" to match TOC entry 4.3. -->
<!-- [FINAL] No doubled words, no homophone errors, no missing words detected. -->
