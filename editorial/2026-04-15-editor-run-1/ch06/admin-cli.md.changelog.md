# Changelog: admin-cli.md

## Pass 1: Structural / Developmental
- 5 comments. Themes:
  - Clean problem → design → code → usage → takeaways arc; good rhythm.
  - Import block shows GORM + bcrypt, but the code a few lines later references `pkgdb.Open` without importing it. Either the import listing is incomplete or the helper should be dropped for a direct `gorm.Open(postgres.Open(dsn), …)` call.
  - "Why no `AutoMigrate`?" sidebar is the section's best moment — didactic and concrete.
  - Idempotent-upsert block uses `result.Error == nil` as the existence test and treats every other error as "not found"; worth either tightening to `errors.Is(result.Error, gorm.ErrRecordNotFound)` or acknowledging the simplification in prose.
  - "Kubernetes operators" is ambiguous (the Operator pattern vs. humans running Kubernetes); please clarify.

## Pass 2: Line Editing
- **Line ~19:** remove tautology
  - Before: "adding one would create a security surface area that needs protection"
  - After: "adding one would create a security surface that needs protection"
  - Reason: "surface area" is redundant — the surface *is* the area.
- **Line ~68:** soften overclaim
  - Before: "Go's `flag` package is intentionally simple."
  - After: "Go's `flag` package is intentionally minimal."
  - Reason: "simple" is on the cut-list; "minimal" is more precise.
- **Line ~70:** hedge a factual comparison
  - Before: "If you are coming from Java/Kotlin, this is the equivalent of a bare-bones `args` parser."
  - After: "If you are coming from Java/Kotlin, this is roughly equivalent to a bare-bones `args` parser."
  - Reason: Go's `flag` handles `--flag=value`, help text, and defaults; a bare `args` parser does not.
- **Line ~88:** remove filler
  - Before: "The CLI just assumes the schema is in place."
  - After: "The CLI assumes the schema is in place."
  - Reason: "just" is in the cut-list.
- **Line ~136:** remove filler
  - Before: "If you forget your admin password, just re-run it with a new one."
  - After: "If you forget your admin password, re-run it with a new one."
  - Reason: "just" is in the cut-list.
- **Line ~144:** register consistency
  - Before: "the auth database is exposed on port 5434 (as defined in `deploy/.env`):"
  - After: "the auth database is exposed on port 5434 (set in `deploy/.env`):"
  - Reason: "set in" is tighter and matches the chapter's prose register.

## Pass 3: Copy Editing
- **Chapter-wide:** replace ASCII `--` with Unicode em dash `—` (no surrounding spaces). CMOS 6.85.
- **Lines ~70, ~182:** "cobra" → "Cobra" in prose. CMOS 8.154 product capitalization; the library's canonical branding uses the capital form.
- **Line ~138:** "`Save` -- `First`" → "`Save` — `First`". CMOS 6.85.
- **Line ~23:** please verify the intended sense of "Kubernetes operators" — reads as the Operator pattern (CRD + controller), but context implies humans running kubectl. Recommend "In Kubernetes, teams often run one-off jobs…".
- **Query — line ~48:** please verify import completeness. The imports block lists `gorm.io/driver/postgres` and `gorm.io/gorm`, but line 80 calls `pkgdb.Open(dsn, pkgdb.Config{})`. Reconcile: either add `"github.com/fesoliveira014/library-system/pkg/db"` to the import block, or switch the code to `gorm.Open(postgres.Open(dsn), &gorm.Config{})`.
- **Query — line ~80:** please verify that `pkgdb.Config{}` (zero value) triggers the same pool defaults used elsewhere in the auth service. The narrative claim depends on this.
- **Query — line ~93:** please verify `bcrypt.DefaultCost == 10` — confirmed in `golang.org/x/crypto/bcrypt`.
- **Line ~86:** please verify the cross-reference `../ch02/repository-pattern.md#configuring-the-connection-pool` resolves to the expected heading slug.

## Pass 4: Final Polish
- No typos, doubled words, or missing words found in prose.
- Cross-ref to Chapter 2 on line 86 should resolve under standard mkdocs slug rules; verify once rendered.
- The blockquote on line 88 uses a Unicode em dash — confirming the author knows the character exists, which strengthens the case for globalizing it across the chapter.
