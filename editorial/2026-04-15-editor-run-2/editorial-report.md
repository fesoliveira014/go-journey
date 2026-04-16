# Editorial Report — Run 2 (2026-04-15)

## Scope

Full manuscript review: Introduction + 14 chapters, 70 sections, ~24,400 lines.
Previous run (run-1) ignored per author request.

## Aggregate Statistics

| Category | Count |
|----------|-------|
| Structural comments | ~35 |
| Line edits | ~120 |
| Copy edits | ~180 |
| Factual queries | ~55 |

## Global Patterns (apply across all chapters)

### 1. Em Dash Formatting (HIGH — ~150+ instances)

Nearly every file uses ` -- ` (spaced double hyphen) where CMOS 6.85 requires `—` (em dash, no spaces). Some files (ch03, parts of ch10/index.md) use the correct `—`, creating inconsistency. A few files mix both forms within the same document (ch07/reservation-service.md).

**Fix:** Batch-replace all prose instances of ` -- ` with `—`. Do not change `--` inside code blocks, YAML, or command-line examples. Also remove spaces around `—` in reference/footnote lines that use ` — `.

### 2. "trade-off" vs "tradeoff" (~10 instances)

CMOS prefers the hyphenated noun "trade-off." The unhyphenated "tradeoff" appears in ch02, ch07, ch08, and elsewhere.

**Fix:** `"tradeoff"` → `"trade-off"` and `"tradeoffs"` → `"trade-offs"` globally.

### 3. Go 1.26 in Dockerfiles (ALL chapters with Dockerfiles)

`golang:1.26-alpine` appears in Dockerfiles across chapters 3, 5, 10, and elsewhere. Go 1.26 does not exist as of the knowledge cutoff (latest stable: Go 1.24). If the book targets a Feb 2026 publication date, Go 1.26 is plausible but unverifiable. The chapter prerequisites (ch01/index.md) also reference "Go 1.26+."

**Action:** Author must verify at publication time. Consider adding a note in the introduction that version numbers should be checked against current releases.

### 4. Alpine 3.19 in Dockerfiles

Alpine 3.19 (released Dec 2023) appears in multiple Dockerfiles. Alpine 3.21+ is available. Same publication-date concern as Go versions.

**Action:** Verify at publication time.

### 5. Filler Preambles (~15 instances)

The phrase pattern "A few things/details/decisions are worth noting/calling out/explaining" recurs across chapters 2, 5, 6, 8, 12, 13, and 14. In each case, the sentence adds no information — the details follow regardless.

**Fix:** Delete the preamble sentence and proceed directly to the first detail.

### 6. Heading Capitalization Inconsistency

Some headings use title case ("Image Tagging Strategy," "Why RDS Over StatefulSets in Production"), while most use sentence case. The manuscript should standardize on one convention.

**Fix:** Convert all to sentence case for consistency with the majority of headings.

### 7. Capitalize After Colon (CMOS 6.63) (~8 instances)

Several instances where a complete sentence follows a colon but begins with a lowercase letter. Examples: ch05/templates-htmx.md L325, ch05/admin-crud.md L187, ch02/wiring.md L141.

**Fix:** Capitalize the first word after a colon when what follows is a complete sentence.

### 8. Numbers in Prose (CMOS 9.2) (~15 instances)

Numbers under one hundred in running prose sometimes appear as numerals (16, 30, etc.) instead of being spelled out. Numerals with units (14 KB, 5 MB, 100 ms) are correct as-is.

**Fix:** Spell out numbers under one hundred in prose that do not have units attached. Tables and code output are exempt.

### 9. British vs American Spelling (2 instances)

"serialises" (ch07/event-driven-architecture.md L222, ch12/preparing-services.md L162) and "recognise" (ch01/http-server.md L226) are British spellings in an otherwise American English manuscript.

**Fix:** `"serialises"` → `"serializes"`, `"recognise"` → `"recognize"`, `"initialises"` → `"initializes"`.

### 10. `grpc.DialContext` vs `grpc.NewClient` Inconsistency (Ch11)

ch11/grpc-testing.md uses `grpc.NewClient` (current, non-deprecated API as of grpc-go v1.63.0), while ch11/e2e-testing.md uses `grpc.DialContext` (deprecated). These should be consistent.

**Fix:** Use `grpc.NewClient` throughout.

### 11. "AKE" → "AKS" (Ch12)

Azure Kubernetes Service is abbreviated "AKS," not "AKE." Appears in ch12/index.md and ch12/kind-setup.md.

### 12. `aws_rds_cluster` vs `aws_db_instance` Confusion (Ch13–14)

Section 13.4 defines `aws_db_instance` (standard RDS), but sections 13.9 and 14.5 reference `aws_rds_cluster` (Aurora). All Aurora references should be corrected to standard RDS.

### 13. Variable Naming: `var.region` vs `var.aws_region` (Ch13)

Section 13.1 uses `var.region`; sections 13.2, 13.6, and 13.8 use `var.aws_region`. Unify throughout.

### 14. Import Path Inconsistency (Ch11)

Some files use `fesoliveira014/library-system`; ch11/e2e-testing.md uses `yourorg/library`. Standardize.

### 15. Footnote Marker Placement (scattered)

CMOS 14.21 places footnote markers after terminal punctuation. At least one instance (ch04/auth-fundamentals.md L163) has the marker before the period.

### 16. Reference Line Formatting

Footnote/reference descriptions use mixed separator styles: some use ` -- `, some use ` — ` (spaced em dash). Standardize to `—` (no spaces) matching body text.

## Chapter-Level Observations

### Strongest chapters (cleanest prose, best structure)
- **Ch01** — Clean, well-paced, good cross-language comparisons
- **Ch11** — Thorough testing coverage, excellent pedagogical structure
- **Ch09** — Strong observability content, clear diagrams

### Chapters needing most attention
- **Ch13** — Multiple factual inconsistencies (Aurora/RDS confusion, variable naming, security group duplication, incorrect forward references)
- **Ch07** — Mixed em dash styles within files, British spelling
- **Ch14** — Forward reference errors (section numbers), ACM renewal timing

### Voice and tone
The tutorial voice is consistent and effective throughout. The Java/Spring comparisons are well calibrated for the target audience. Contractions are used consistently and appropriately for the register. The author's voice is strong — editorial changes should tighten without homogenizing.

### Code quality
Code blocks are generally well formatted with appropriate language tags. Key issues:
- Missing `encoding/json` import in ch01/testing.md
- Missing `pkgdb` import in ch06/admin-cli.md
- `setFlash` vs `s.setFlash` inconsistency between ch05/session-management.md and ch05/admin-crud.md
- `TestMain` with manually constructed `*testing.T` in ch11/kafka-testing.md (will panic or misbehave)
- Non-existent `aws_msk_topic` Terraform resource in ch13/msk.md
