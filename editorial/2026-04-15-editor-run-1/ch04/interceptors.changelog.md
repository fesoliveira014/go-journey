# Changelog: interceptors.md

## Pass 1: Structural / Developmental
- 6 comments. Themes: strong opener and Java-analogy framing; end-to-end grpcurl test sequence is the chapter's best demonstration artifact; the "Why Typed Context Keys?" sub-subsection is the pedagogical model to imitate elsewhere; consider elevating the "interceptor-generic / handler-specific" bullets to a callout.

## Pass 2: Line Editing
- **Line ~32:** Count mismatch.
  - Before: "Our auth interceptor does all three: it rejects requests without valid tokens, extracts user info from the token, injects it into the context, and then calls the handler."
  - After: "Our auth interceptor does three of these: it rejects requests without valid tokens, injects user info into the context, and then calls the handler."
  - Reason: The prior bullet list has four actions; the sentence lists four but says "all three". Tighten and correct.
- **Line ~38:** Split dense sentence.
  - Before: "The `pkg/auth` directory is a **separate Go module** (with its own `go.mod`) that services import via `replace` directives in the workspace."
  - After: "The `pkg/auth` directory is a **separate Go module** with its own `go.mod`. Services import it via `replace` directives in the workspace."
  - Reason: Two short sentences read cleaner.
- **Line ~111:** Drop "obviously".
  - Before: "`Register` and `Login` obviously cannot require a token (the user does not have one yet)."
  - After: "`Register` and `Login` cannot require a token — the user does not have one yet."
  - Reason: "Obviously" is subtly condescending.
- **Line ~152:** Drop "literally".
  - Before: "Other packages literally cannot construct a key that would match"
  - After: "Other packages cannot construct a key that matches"
  - Reason: "Literally" is filler; the sentence is strong without it.
- **Line ~195:** Drop "obviously" in second instance.
  - Before: "`Register`, `Login`, and OAuth RPCs are obviously public."
  - After: "`Register`, `Login`, and the OAuth RPCs are public by necessity."
- **Line ~200:** Verify "would" conditional.
  - Before: "skips authentication for read operations and the availability update (which would be called by an internal service)"
  - After (if confirmed): "...(which is called by an internal service, the reservation worker)"
  - Reason: The conditional "would" sounds hypothetical; if UpdateAvailability is actually called by the reservation service from Ch.3/5, use present tense.
- **Line ~216:** Smoother compound adjective.
  - Before: "because it is business-logic-specific"
  - After: "because it depends on business rules"

## Pass 3: Copy Editing
- **Line ~33:** Consistency — "pkg/auth/" in heading has trailing slash; prose refers to "pkg/auth" without. Pick one.
- **Line ~113:** Verified — gRPC metadata keys are case-insensitive, `md.Get("authorization")` normalizes.
- **Line ~173:** "HTTP 401 vs 403" — CMOS 6.13 takes "vs." with a period. Check consistency across chapter.
- **Line ~173:** Verified — gRPC `Unauthenticated` → HTTP 401; `PermissionDenied` → HTTP 403 per the official gRPC-HTTP/2 spec.
- **Line ~266:** Verified — ISBN-13 "978-0134190440" is correct for "The Go Programming Language" (Donovan & Kernighan, 2015).
- **Line ~343:** Verified — "golang-standards/project-layout" is correctly described as community, not official.

## Pass 4: Final Polish
- **References section:** Same chapter-wide pattern — footnotes defined but not anchored in prose.
- No typos, doubled words, or broken cross-refs found.
- Forward reference to "Chapter 6" in grpcurl section — confirm the CLI promotion utility is actually planned for Ch. 6.
