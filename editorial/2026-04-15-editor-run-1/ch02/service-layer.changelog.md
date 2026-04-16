# Changelog: service-layer.md

## Pass 1: Structural / Developmental
- 6 comments. Themes:
  - Section is among the strongest in the chapter — clear separation of concerns, explicit Java/Kotlin bridge, TDD-style exercise.
  - Opportunity to explicitly name Dependency Inversion Principle for extra SOLID-anchor payoff.
  - Mock/fake terminology is slightly imprecise (the sample is a stateful fake, not a behavior-verifying mock) — worth a brief taxonomy note.
  - Inconsistency: body text claims the repository "would happily insert a book with...negative availability", but §2.2 introduces a CHECK constraint that rejects exactly that. Reconcile.
  - Exercise solution glosses a TOCTOU race (GetByID → Delete) that will matter in later chapters; flag as forward reference.
  - "bug" in DeleteBook framing is slightly loaded — consider "incomplete" or "gap in invariants".

## Pass 2: Line Editing
- **Line ~5:** "and it depends on neither" — technically inaccurate.
  - Before: "it depends on neither"
  - After: "it does not depend on gRPC or GORM — it sees both only through interfaces it owns or is handed"
- **Line ~24:** Remove banned "simply".
  - Before: "a type satisfies an interface simply by having..."
  - After: "a type satisfies an interface by having..."
- **Line ~42:** "This is a deliberate inversion..." — optional, but naming the SOLID DIP would anchor the pattern for the reader.
- **Line ~62:** Remove banned "just".
  - Before: "no framework, just a constructor"
  - After: "no framework, a constructor"
- **Line ~65:** Remove banned "simply".
  - Before: "`GetBook` simply delegates"
  - After: "`GetBook` delegates"
- **Line ~72:** Split 50-word sentence.
  - Before: "There's no business logic here worth adding a layer for — but the layer still matters because it's the right place for logic *when it does exist*, and it decouples the gRPC handler from the repository interface."
  - After: "There's no business logic here worth a dedicated layer for. The layer still matters — it's the right place for logic *when it exists*, and it decouples the gRPC handler from the repository interface."
- **Line ~86:** Reconcile with §2.2 CHECK constraint.
  - Before: "it would happily insert a book with a blank title and negative availability if asked"
  - After: "the repository doesn't validate either rule; the database CHECK constraints catch the most egregious violations, but defensive validation in the service layer produces better error messages and prevents an RPC round-trip"
- **Line ~119:** Tighten "inspect which type of error".
  - Before: "Callers can inspect which type of error they're dealing with using `errors.Is`:"
  - After: "Callers can check the type using `errors.Is`:"
- **Line ~128:** Remove banned "just".
  - Before: "It just checks..."
  - After: "It checks..."
- **Line ~130:** Remove banned "just".
  - Before: "just wrap a sentinel, unwrap it with `errors.Is`"
  - After: "wrap a sentinel, unwrap it with `errors.Is`"
- **Line ~140:** Remove banned "just".
  - Before: "A mock just needs to implement..."
  - After: "A mock needs only to implement..."
- **Line ~208:** Soften "bug".
  - Before: "This has a bug:"
  - After: "This is incomplete:" or "This leaves a gap in the invariants:"

## Pass 3: Copy Editing
- **Line ~86:** "defence" — British spelling; standardize to US "defense" for chapter consistency.
- **Line ~119:** "title is required" — straight quotes in Markdown source; verify build pipeline renders curly in output.
- **Line ~130, 193:** Leading space before `[^2]` and `[^1]` footnote markers. Remove the space (CMOS/Markdown convention).
- **Line ~171:** Taxonomy note — strictly, the sample is a stateful fake per xUnit Patterns terminology. Either acknowledge or rename "Hand-Written Fakes"; calling the class `mockBookRepository` is fine, prose-level framing is where the looseness bites.
- **References:** Verify footnote URLs; consider linking to the 2019 Go 1.13 errors package post as supplementary reading.

### Factual queries (Please verify)
- **Line ~86:** Database CHECK constraint interplay with service-layer validation claim.
- **Line ~130, 193:** Leading-space footnote convention in the house style.
- **References:** Current canonical URL for "Error handling and Go" blog post, and whether a newer post supersedes it.

## Pass 4: Final Polish
- No typos, doubled words, or homophone errors found.
- Cross-reference to Chapter 7 for TOCTOU race in DeleteBook would land well if the rest of the book uses explicit chapter numbers in forward references.
