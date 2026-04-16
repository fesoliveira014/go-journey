# Changelog: repository-pattern.md

## Pass 1: Structural / Developmental
- 9 comments. Themes:
  - Strong opening framing (ORM + repository).
  - "UpdateAvailability" race-condition treatment is a highlight.
  - Some content overlaps with §2.2 (AutoMigrate warning); suggest cross-reference instead of re-stating.
  - Consider adding a brief note on ILIKE indexing pitfalls for experienced readers.
  - TRUNCATE-vs-transaction-rollback is a live debate in the Go testing community; briefly acknowledge the alternative.
  - Footnotes [^1] and [^2] in references list are not cited inline — cite or drop.
  - Citation style inconsistency: inline hyperlinks ("Error handling and Go", "Don't just check errors") mix with numbered footnotes elsewhere. Standardize.

## Pass 2: Line Editing
- **Line ~25:** Awkward "at GORM's level of maturity" phrasing.
  - Before: "The surface-level difference is Go's lack of generics at GORM's level of maturity..."
  - After: "The surface-level difference is that GORM predates Go generics — you pass `&record` pointers and the ORM uses reflection to locate the target table. This feels awkward at first but becomes second nature."
  - Reason: Go has had generics since 1.18; current phrasing reads dated.
- **Line ~141:** Remove banned "just".
  - Before: "no framework, just a constructor that accepts the dependency"
  - After: "no framework, a constructor that accepts the dependency and nothing more"
- **Line ~235:** "This method is special." — vague filler.
  - After: "This method is the race-condition trap — done wrong, it corrupts availability counts under concurrency."
- **Line ~249:** Imprecise "reject" wording for WHERE-clause filtering.
  - Before: "the database will reject an update that would underflow"
  - After: "the WHERE clause filters out rows that would go negative — so an underflow simply matches zero rows"
- **Line ~296:** Ambiguous "per call".
  - Before: "**`query` is immutable per call.**"
  - After: "**`query` is immutable.**"
- **Line ~121:** "(which is painful — GORM's interface is large)" — register bump.
  - Suggested: "(which is impractical — GORM's API is large)"
- **Line ~343:** Redundancy with §2.2.
  - Suggested: Shorten design-decision #2 with "See §2.2 for the full rationale." and keep only the one-line contrast.

## Pass 3: Copy Editing
- **Line ~97:** "data access code" → "data-access code" (compound modifier, CMOS 7.81).
- **Line ~172:** Inline hyperlinks contradict the footnote-only style used in sibling sections. Convert to footnotes.
- **Line ~191:** Generated-SQL clause order wrong — "WHERE id = ? LIMIT 1 ORDER BY id" should be "WHERE id = ? ORDER BY id LIMIT 1". Query — if literal GORM output capture actual string.
- **Line ~212:** "upsert-style `UPDATE`" — possibly incorrect characterization of `Save` behaviour; Save does a full-field UPDATE, not an upsert. Query.
- **Line ~199:** `map[string]interface{}` — consider mentioning the Go 1.18 alias `any` for a 2026 book.
- **Line ~296:** "side-effects" → "side effects" (noun form, open compound).
- **Line ~298:** "filtered-but-not-yet-paginated" — hyphenation correct but heavy. Consider prose alternative.
- **Line ~341:** "behaviour" — British spelling, conflicts with American spelling used elsewhere. Standardize.
- **Line ~424:** `if err != model.ErrBookNotFound` in exercise solution contradicts the `errors.Is` guidance given in the surrounding prose. Recommend aligning.
- **Line ~446:** Footnotes [^1] and [^2] not cited inline; cite or remove.

### Factual queries (Please verify)
- **Line ~25:** GORM public API generics status as of 2026 — is Find/First/Where chain still reflection-based?
- **Line ~191:** Actual SQL emitted by `db.First(&book, "id = ?", id)` — clause order.
- **Line ~212:** "upsert-style" characterization of `db.Save(book)`.
- **Line ~330:** `err != migrate.ErrNoChange` idiom — same as in §2.2 query.
- **Line ~172:** URLs for Go blog and Dave Cheney blog posts — confirm live.

## Pass 4: Final Polish
- No typos or doubled words. One notable homophone-adjacent: "its" vs "it's" — checked; correct throughout.
- Cross-references to §2.2 are narrative, not anchored; consider adding explicit section links.
