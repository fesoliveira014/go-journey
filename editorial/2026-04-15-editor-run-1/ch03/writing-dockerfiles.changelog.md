# Changelog: writing-dockerfiles.md

## Pass 1: Structural / Developmental
- 4 comments. Themes:
  - Consider moving "The Monorepo Build Context Challenge" earlier, before the line-by-line dissection of the Catalog Dockerfile, so readers have the *why* before parsing multi-dir COPY statements.
  - The Gateway section is intentionally brief; verify that the brevity doesn't let the inconsistency slip (see factual issue below).
  - **Factual consistency bug**: three locations call the Gateway "self-contained" or say it has "no cross-module dependencies," but the Gateway Dockerfile shown copies `gen/`, `pkg/auth/`, and `pkg/otel/`. Reconcile (either the intro, the summary, or both should be rewritten).

## Pass 2: Line Editing
- **Line ~3:** Drop filler "actual."
  - Before: "walk through the actual Dockerfiles"
  - After: "walk through the Dockerfiles"
- **Line ~3:** Fix "self-contained" claim.
  - Before: "the Gateway (which is self-contained)"
  - After: "the Gateway (which imports a smaller subset of shared modules)"
  - Reason: factual accuracy vs. the Dockerfile shown.
- **Line ~51:** Tighten transition.
  - Before: "Let's go through this line by line."
  - After: "Walk through it top to bottom." (optional)
- **Line ~66:** Correct the directory list.
  - Before: "We copy only `gen/` and `services/catalog/`."
  - After: "We copy only `gen/`, `pkg/auth/`, `pkg/otel/`, and `services/catalog/`."
  - Reason: matches the Dockerfile above.
- **Line ~90:** Passive → active.
  - Before: "Dependencies are downloaded while only the module files are in the image."
  - After: "Go downloads dependencies while only the module files are in the image."
- **Line ~103:** Drop "Note that."
  - Before: "Note that `gen/` contains the protobuf-generated Go code..."
  - After: "`gen/` contains the protobuf-generated Go code..."
- **Line ~116:** Break up a 60+-word packed sentence into four shorter sentences covering each runtime instruction. See annotated file for target.
- **Line ~118:** Trim intensifier.
  - Before: "Running as non-root is a fundamental container security practice."
  - After: "Running as non-root is a basic container-security practice."
- **Lines ~126–127:** Bullet list of "needs files from two directories" misses pkg/auth and pkg/otel. Add them.
- **Line ~233:** Drop "it is useful to."
  - Before: "Before using Compose, it is useful to understand manual image building..."
  - After: "Before using Compose, understand manual image building..."
- **Line ~257:** Trim filler intensifier.
  - Before: "That is exactly what Compose solves"
  - After: "That's what Compose solves"
- **Line ~325:** Fix summary bullet to match reality.
  - Before: "The Gateway Dockerfile follows the same pattern but is simpler because it has no cross-module dependencies."
  - After: "The Gateway Dockerfile follows the same pattern with additional runtime-stage copies for templates and static assets."

## Pass 3: Copy Editing
- **Throughout:** Replace `--` with em dash `—` (no spaces) per CMOS 6.85.
- **Throughout:** Range and unit formatting — en dash for number ranges (CMOS 6.78), space between number and unit (CMOS 9.16): `15–25 MB`, `~5 MB`, `~300 MB`.
- **Line ~74:** Capitalization inside quoted directive paraphrase.
  - Before: `"when you encounter an import..."`
  - After: `"When you encounter an import..."` (full sentence inside quotes, CMOS 6.41/13.13)
- **Line ~147:** "tradeoff" — consider "trade-off" for CMOS compliance; keep consistent chapter-wide.
- **Line ~226:** "CSS, JS" — technically fine; CMOS would prefer "CSS and JavaScript" in running prose, but tech conventions accept abbreviations.
- **Line ~66 and ~158:** "inter-module" vs. "intermodule" — CMOS 7.89 leans to closed form for "inter-," but "inter-module" aids readability. Style choice; keep consistent.
- **Line ~160:** "the internet" — lowercase. Correct.
- **Queries (please verify):**
  - `golang:1.26-alpine` — availability and tag correctness at publication.
  - `alpine:3.19` — freshness (3.20, 3.21 exist).
  - Reference URLs (all four).
  - Exact image sizes quoted (15–25 MB, ~300 MB, 400 MB savings): worth measuring at build time on publication rather than hand-waving with round numbers.

## Pass 4: Final Polish
- No typos, doubled words, or homophones detected. Internal cross-references (the Catalog Dockerfile code block vs. the prose dissection) are consistent except for the directory-count drift flagged in Pass 2.
