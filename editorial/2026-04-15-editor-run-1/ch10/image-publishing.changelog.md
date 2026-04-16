# Changelog: image-publishing.md

## Pass 1: Structural / Developmental

- 6 comments. Themes:
  - Strong section order; the Design Decision (Dockerfiles vs Earthly) section is the pedagogical peak.
  - Some duplication with 10.3 (permissions, build-and-push job listing). Acceptable for a reference-style section, but open with a cross-reference.
  - Forward pointer earlier in the chapter needed: the reader should know from 10.0/10.3 that publishing uses Dockerfiles (not Earthly) because of provenance/SLSA features; 10.5 explains it.
  - Claim "The build logic is identical to what the Dockerfile does" without showing the Dockerfile is weakly supported. Soften or show.
  - `fail-fast` paragraph contains an internal contradiction: first claims the other four jobs continue, then says `fail-fast: true` cancels them. Must be fixed.
  - Exercise set varies well from observation to construction (release workflow).

## Pass 2: Line Editing

- **Line ~21:** Split 67-word sentence on `latest` reproducibility.
  - Before: "The problem is that `latest` is not reproducible. If you pull on Monday and a colleague pulls on Wednesday after a new push, you are running different code under the same tag. In production, if a deployment controller reads `imagePullPolicy: Always` and the pod restarts, it may pull a different image than the one running in other replicas. This is a support and debugging nightmare."
  - After: "The problem is that `latest` is not reproducible. Pull on Monday, pull on Wednesday after a new push, and you are running different code under the same name. In production, a deployment controller with `imagePullPolicy: Always` may pull a different image than other replicas when a pod restarts — a support and debugging nightmare."
  - Reason: Active voice; 18 words shorter; rhythm improves.

- **Line ~125:** Rewrite the `fail-fast` paragraph to remove the internal contradiction.
  - Before: "If one matrix job fails (e.g., the `search` Dockerfile has a bug), the other four continue. GitHub marks the overall `build-and-push` job as failed when any matrix job fails, but the other images are still pushed. This is the default `fail-fast: true` behavior — GHA will cancel remaining matrix jobs if one fails. You can set `fail-fast: false` if you want all matrix jobs to always run to completion regardless."
  - After: "If one matrix job fails (for example, the `search` Dockerfile has a bug), GitHub Actions' default `fail-fast: true` behavior cancels any in-flight matrix jobs that have not yet completed. Jobs that had already finished still pushed their images. GitHub marks the overall `build-and-push` job as failed. Set `fail-fast: false` if you want every matrix job to run to completion regardless of failures."
  - Reason: Removes contradiction; states the default behavior correctly.

- **Line ~152:** Split 60-word sentence on `context: .`.
  - Before: "`context: .` sets the Docker build context to the repository root. This is required because the Dockerfiles COPY files from directories outside their own service directory — specifically `gen/` (generated protobuf code) and `pkg/` (shared libraries). If the context were set to `services/catalog/`, the `COPY gen/ ./gen/` instruction would fail with a 'file not found' error."
  - After: "`context: .` sets the Docker build context to the repository root. It has to be, because the Dockerfiles `COPY` files from outside their service directory — specifically `gen/` (generated protobuf code) and `pkg/` (shared libraries). With a narrower context like `services/catalog/`, `COPY gen/ ./gen/` would fail with 'file not found'."
  - Reason: Trims 10 words; backticks on `COPY` consistent with elsewhere.

- **Line ~221:** Split 66-word sentence on `sha-<commit>` immutability.
  - Before: "The `sha-<commit>` Docker tag does the same thing but enforces immutability: you cannot overwrite an existing tag if you configure GHCR to prevent it, and in practice no one does."
  - After: "The Docker `sha-<commit>` tag does the same, but with a stronger convention of immutability: you cannot overwrite an existing tag (when GHCR is configured to prevent it), and in practice no one does."
  - Reason: Softens "enforces" (conditional), improves flow.

## Pass 3: Copy Editing

- **Line 162 (heading):** "Dockerfiles vs Earthly" — "vs" without period. Earlier chapter text uses "vs." Be consistent. CMOS 10.42 allows either; pick one.
- **Line 44:** "SemVer" introduced without gloss. CMOS 10.3. Recommend: "the SemVer (Semantic Versioning) tag".
- **Line 172:** "SLSA" acronym — define on first use. Recommend: "SLSA (Supply-chain Levels for Software Artifacts)".
- **Line 111:** "(see Section 10.4)" — Section 10.4 is linting; the `+ci` target is defined in 10.2. Recommend: "(see Sections 10.2 and 10.4)".
- **Line 178:** "(as of writing)" — add year or version: "(as of early 2026)" or "(as of Earthly 0.8.x)".
- **Line 208:** "docker compose" — distinguish CLI (`docker compose`, code) from product name ("Docker Compose", prose).
- **Line 219:** "`immutable versioned artifacts`" — Maven snapshots are mutable. Recommend: "both authenticate callers, store versioned artifacts (immutable in the case of release versions), and provide a pull endpoint."
- **Line 225:** "`./gradlew publishAll`" — `publishAll` is not a canonical Gradle task; it is user-defined. Recommend referencing `./gradlew publish` (aggregate) or the Maven Publish plugin tasks.
- Serial commas consistent (CMOS 6.19). Good.
- Em dashes used consistently (CMOS 6.85); fewer `--` occurrences than in earlier files.
- Compound adjectives hyphenated correctly ("human-readable", "open-source", "pull-and-run", "pull-request", "40-character", "multi-platform", "multi-module"). Good.
- Please verify URLs in references; GitHub docs URLs shift.
- Please verify `alpine:3.19`, `docker/build-push-action@v6`, `docker/login-action@v3`, `earthly --push` / `--image` flag semantics at 2026-04-15.

## Pass 4: Final Polish

- **Line ~125:** Factual contradiction on `fail-fast` behavior (fix in Pass 2). Highest priority.
- **Line ~225:** `publishAll` is not a canonical Gradle task (see Pass 3).
- **Line ~219:** "Immutable" claim for Maven is inaccurate for snapshots (see Pass 3).
- **Line ~221:** "Enforces immutability" overstates the default behavior (see Pass 2).
- No doubled words or obvious typos detected.
- No broken cross-references (though Section 10.4 reference is miscategorized; see Pass 3).
