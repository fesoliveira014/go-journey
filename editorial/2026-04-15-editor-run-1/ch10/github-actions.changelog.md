# Changelog: github-actions.md

## Pass 1: Structural / Developmental

- 7 comments. Themes:
  - Section order (concepts → marketplace → PR workflow → main workflow → why two workflows → JVM comparisons → exercises) is strong.
  - "Why a Separate Workflow for PRs?" and "Why Two Workflows" cover overlapping ground. Consider consolidating. The second section's unique value is the security/permissions framing for fork PRs.
  - Forward pointer needed where `docker/build-push-action` is introduced: the chapter has emphasized Earthly, so an explicit note ("see 10.5 for why publishing uses Dockerfiles rather than Earthly") will save the reader confusion.
  - Supply-chain pinning paragraph should mention commit-SHA pinning as the stricter, supply-chain-safer option.
  - Vocabulary table, Actions table, context-variable table, JVM comparison table all work — dense but readable.
  - "Pure GitHub Actions Alternative" blockquote is a good side-by-side teaching device. Keep.

## Pass 2: Line Editing

- **Line ~111:** Split long blockquote sentence.
  - Before: "This is simpler to read and has fewer moving parts. The downside: it only works in CI. You cannot run `actions/setup-go` locally. If lint fails in CI, you reproduce it by pushing another commit and waiting for a runner -- there is no `earthly +lint` equivalent on your terminal. The Earthly approach trades a small amount of workflow complexity for full local reproducibility."
  - After: "This is simpler to read and has fewer moving parts. The downside: it only works in CI. You cannot run `actions/setup-go` on your laptop. If lint fails in CI, you reproduce it by pushing another commit and waiting for a runner. The Earthly approach trades a little workflow complexity for full local reproducibility."
  - Reason: Breaks a 65-word chain into cleaner short sentences and trims filler ("a small amount of").

- **Line ~184:** Split 60-word sentence.
  - Before: "Identical to the PR workflow. The same lint and test checks run on `main` even after merging. This catches the rare case where a merge introduces a conflict that was not caught in the PR (for example, two PRs that individually pass but conflict when both land on `main`). Running CI again on `main` is a small cost for a meaningful safety net."
  - After: "Identical to the PR workflow. The same lint and test checks run on `main` even after merging. This catches the rare case where a merge introduces a conflict that was not caught in the PR — for example, two PRs that individually pass but conflict when both land on `main`. Running CI again on `main` is a small cost for a meaningful safety net."
  - Reason: Em dashes replace the parenthetical; rhythm improves.

- **Line ~229:** Split 84-word "Earthly-Push Alternative" blockquote.
  - Before: single run-on sentence plus closing.
  - After: break after "…built-in integrations." — make each integration its own fragment or a bulleted list, then the two-line "learning/production" conclusion.
  - Reason: Readability; the list of lost integrations deserves visual separation.

## Pass 3: Copy Editing

- **Throughout:** `--` → em dash `—` (CMOS 6.85).
- **Line 3:** "section 10.2" → "Section 10.2" for consistency (CMOS 8.179; internal consistency).
- **Line 176:** "GitHub Container Registry" — acronym GHCR glossed elsewhere (10.0, 10.5) but not in this file on first use. Current text ("GitHub Container Registry (`ghcr.io`)") is acceptable; consistent with a later-use-acronym style.
- **Line 260:** "Jenkins master" — per Jenkins project's 2020 terminology change, prefer "Jenkins controller".
- **Line 214:** `github.repository` note — the variable itself preserves case; only GHCR lowercases. Text says "in lowercase" as if the variable is lowercase. Recommend: "`owner/repo` (GHCR requires lowercase; `github.repository` preserves the original case)."
- **Line 44–47 (Actions table):** Please verify all action versions are current:
  - `actions/checkout@v4`
  - `earthly/actions-setup@v1`
  - `docker/login-action@v3`
  - `docker/build-push-action@v6`
- **Line 72:** Please verify: Earthly `v0.8.15` is current release.
- **Line 107:** Please verify: `actions/setup-go@v5`, `golangci/golangci-lint-action@v6`, `go-version: '1.26'`.
- **References:** Please verify all three GitHub docs URLs still resolve.
- Serial commas: consistent (CMOS 6.19). Good.
- e.g. / i.e.: rendered with comma after per CMOS 6.43. Good.
- Compound adjectives: "pure-GHA", "cloud-managed", "supply-chain", "40-character", "quality-of-life", "open-source" all hyphenated before nouns (CMOS 7.81). Good.
- HTTP status code "403" as numeral is correct (CMOS 9.13 technical measurements).

## Pass 4: Final Polish

- **Line 215:** Verify `github.repository` case behavior (minor factual nuance).
- **Line 260:** "Jenkins master" — inclusive-language update recommended.
- **Line 201:** "services may reference shared code or the root `go.mod`" — technically, the chapter has described services as having replace-directed local modules with their own `go.mod`, not sharing a root `go.mod`. Please verify: is there a repository-root `go.mod`, or does each service have its own? If the latter, this sentence is misleading.
- No typos, doubled words, or homophone issues detected.
