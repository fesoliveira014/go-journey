# Changelog: dev-workflow.md

## Pass 1: Structural / Developmental
- 3 comments. Themes:
  - Consider moving the sequence diagram up to just after "How Air Works" so the reader's mental model is cemented before the `.air.toml` and Dockerfile detail.
  - The "When to Rebuild vs. Restart" table is the most useful artifact in the chapter — preserve.
  - Flagged a significant consistency concern: the `Dockerfile.dev` snippets for catalog and gateway appear to omit `pkg/auth` and `pkg/otel`, which the production Dockerfiles (3.2) copy. If those modules are imported by the services, the dev image build will fail. This is either a manuscript bug or a real property of the current files; should be reconciled.

## Pass 2: Line Editing
- **Line ~3:** Tighten contrast.
  - Before: "That is correct for deployment, but painful for development:"
  - After: "Correct for deployment, painful for development:"
- **Line ~3:** Active voice.
  - Before: "code changes are automatically detected and rebuilt"
  - After: "your code changes trigger automatic rebuilds"
- **Line ~42:** Active voice / cleaner phrasing.
  - Before: "Everything else ... is inherited from the base `docker-compose.yml`."
  - After: "Everything else comes from the base `docker-compose.yml`."
- **Line ~117:** Minor passive fix.
  - Before: "Air is installed with `go install`."
  - After: "Air installs via `go install`."
- **Line ~119:** Drop mild filler.
  - Before: "since the container already has the necessary libraries"
  - After: "since the container has the necessary libraries"
- **Line ~141:** Prefer precise noun.
  - Before: "The magic is in the volume mounts"
  - After: "The key is in the volume mounts"
- **Line ~151:** Idiomatic placement of "only."
  - Before: "the container would only have the source code that was `COPY`ed"
  - After: "the container would have only the source code that was `COPY`ed"
- **Line ~151:** Active voice.
  - Before: "Changes on your host would not be reflected."
  - After: "Changes on your host would not appear in the container."
- **Line ~190:** Active.
  - Before: "You will see Air's output in the logs:"
  - After: "Air's output appears in the logs:"
- **Line ~324:** Active summary bullet.
  - Before: "Source code changes are handled by Air automatically."
  - After: "Air handles source-code changes automatically."

## Pass 3: Copy Editing
- **Throughout:** Replace `--` with em dash `—` (no spaces) per CMOS 6.85.
- **Line ~85:** "1000ms" — space unit: "1000 ms" or "1,000 ms" (CMOS 9.16). Keep consistent with rest of chapter's unit style.
- **Line ~58:** Bullet parallelism in "How Air Works" list: items have mixed syntactic lead; consider rewriting all as subject-verb.
- **Line ~249:** Inconsistency — "A local PostgreSQL installation on port 5433" is atypical (default Postgres port is 5432). Clarify.
- **Line ~289:** Term inconsistency — "health check response body" (open) vs. the rest of the chapter using "healthcheck" (closed). Pick one style for prose and stick with it (the YAML key `healthcheck:` is fixed).
- **Line ~310:** "WSL2" vs. Microsoft's "WSL 2" — project style choice.
- **Line ~311:** Range — "1-3 seconds" → "1–3 seconds" (en dash, CMOS 6.78).
- **Queries (please verify):**
  - `Dockerfile.dev` file content — do the real files in the repo omit `pkg/auth`/`pkg/otel`? If yes, they will break the dev build.
  - `github.com/air-verse/air@latest` — canonical module path (was `cosmtrek/air`; renamed to `air-verse/air` in 2024).
  - Reference URLs (all four).

## Pass 4: Final Polish
- No typos, doubled words, or broken cross-references. Numbers and ranges flagged in Pass 3. Main open issue is the possible manuscript bug around Dockerfile.dev shared-module copies, flagged for author.
