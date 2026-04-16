# Changelog: auth-fundamentals.md

## Pass 1: Structural / Developmental
- 5 comments. Themes: strong opener and pacing; strengthen the "alg: none" explanation with one sentence on *why* the HMAC check defends; "When to Use Each" bullets would benefit from example contexts; bcrypt subsection could explicitly tie back to the SHA-256 motivation.

## Pass 2: Line Editing
- **Line ~3:** Cut filler "just".
  - Before: "the Go implementations are just more explicit about what is happening under the hood"
  - After: "the Go implementations are more explicit about what happens under the hood"
  - Reason: Style guide cuts "just"; "what happens" is active.
- **Line ~11:** Tighten ambiguous referent.
  - Before: "you are in good company -- but it is wrong."
  - After: "you have company -- but you are wrong."
  - Reason: "It" referent unclear; direct subject is clearer.
- **Line ~15:** Reposition "without salting" for clarity.
  - Before: "They are deterministic without salting. The SHA-256 hash of "password123" is the same on every machine."
  - After: "They are deterministic. Without salting, the SHA-256 hash of "password123" is identical on every machine."
  - Reason: Moves the conditional ("without salting") to govern the example sentence explicitly.
- **Line ~25:** Tighten redundancy.
  - Before: "it generates and embeds a salt automatically"
  - After: "it embeds a random salt automatically"
  - Reason: "Generates and embeds" is doubled; "random" carries the salient property.
- **Line ~57:** Softer, more precise latency claim.
  - Before: "This takes roughly 100ms on modern hardware -- fast enough that users don't notice, slow enough that brute-force is impractical."
  - After: "This takes roughly 100ms on modern hardware -- fast enough that users don't notice, slow enough to make brute-force attacks impractical."
  - Reason: "brute-force" is an adjective; "brute-force attacks" uses the noun form correctly (CMOS 7.81).
- **Line ~105:** Drop "just".
  - Before: "Anyone can decode the header and payload (they are just base64)"
  - After: "Anyone can decode the header and payload (they are plain base64)"
- **Line ~163:** Sharpen stakes.
  - Before: "This is a real vulnerability class"
  - After: "This is a real vulnerability class, not a theoretical concern"
- **Line ~169:** Active voice.
  - Before: "If you have built web applications with Spring Security, you are used to `HttpSession`."
  - After: "If you have built web applications with Spring Security, `HttpSession` will feel familiar."
- **Line ~171:** Drop "just".
  - Before: "it just validates the signature and reads the claims"
  - After: "it validates the signature and reads the claims"

## Pass 3: Copy Editing
- **Line ~33:** Query — bcrypt encoded format: confirm "22-char salt + 31-char hash" split is accurate per bcrypt spec.
- **Line ~57:** Query — confirm `bcrypt.DefaultCost = 10` in `golang.org/x/crypto/bcrypt` (it is, at time of writing; include a note that this may change in future releases).
- **Line ~95:** Query — `sub` claim gloss as "user ID" is application convention, not RFC 7519 text.
- **Line ~119:** Query — confirm `github.com/golang-jwt/jwt/v5` is current maintained major version.
- **Line ~220:** Query — "Mazieres" should be "Mazières" if the book supports non-ASCII in references.
- **Throughout:** Style-pass flag on "--" dash convention (consistent with chapter but worth confirming SSG renders them as em/en dashes per CMOS 6.85 / 6.78).

## Pass 4: Final Polish
- No typos, doubled words, or broken cross-refs found.
- Footnote anchors (`[^1]`...`[^5]`) all resolve.
