# Changelog: auth-service.md

## Pass 1: Structural / Developmental
- 6 comments. Themes: good layered walkthrough; consider justifying `ValidateToken`'s presence given the interceptor path; main.go snippet references undefined symbols (googleClientSecret, interceptor) — add brevity comment; callout on `log.Fatal` is excellent and should be the model for future design-decision asides.

## Pass 2: Line Editing
- **Line ~22:** 41-word sentence, split.
  - Before: "`ValidateToken` is used by other services (or a gateway) to verify a token and extract the user ID and role without needing the JWT secret themselves -- though in our architecture, services share the secret and validate locally via the interceptor."
  - After: "Other services (or a gateway) call `ValidateToken` to verify a token and extract the user ID and role without holding the JWT secret themselves. In our architecture, however, services share the secret and validate locally via the interceptor."
  - Reason: Two sentences read more cleanly; removes "used by" passive.
- **Line ~75:** Tighter modal.
  - Before: "A Google user with ID `12345` can only have one row."
  - After: "A Google user with ID `12345` has only one row."
  - Reason: "can only have" suggests permission; constraint guarantees the fact.
- **Line ~75:** Slightly more direct.
  - Before: "For our learning project, we keep it simple."
  - After: "For this learning project, we keep it simple."
- **Line ~203:** Restructure parallel failure clauses.
  - Before: "whether the email does not exist, the user is OAuth-only, or the password is wrong, the same error is returned -- `ErrInvalidCredentials`."
  - After: "all three failure paths -- email not found, OAuth-only user, wrong password -- return the same error, `ErrInvalidCredentials`."
  - Reason: The "whether...or...or" construction is clunky; the list form with em dashes is clearer.

## Pass 3: Copy Editing
- **Line ~124:** Query — confirm PostgreSQL SQLSTATE "23505" = `unique_violation` (it is; documented in PostgreSQL Appendix A. Errors).
- **Line ~184:** "e.g. database failures" in code comment — CMOS 6.43 requires comma after "e.g.,". Low priority inside a Go source comment; leave as author preference.
- **Throughout:** "four-layer", "database-level", "base64url-encoded" — compound adjectives all correctly hyphenated before nouns (CMOS 7.81). Good.

## Pass 4: Final Polish
- **References section:** Footnote anchors `[^1]`–`[^5]` are defined but none are referenced in the body text of this file. Either insert anchors in the prose (e.g., attach [^1] to the "gRPC `AlreadyExists`" mention) or treat the section as a bibliography. Flagged for author.
- No typos, doubled words, or broken cross-refs found.
- "e.g." and "e.g.," usage consistent elsewhere.
