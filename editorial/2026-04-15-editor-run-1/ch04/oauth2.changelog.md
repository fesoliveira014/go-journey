# Changelog: oauth2.md

## Pass 1: Structural / Developmental
- 5 comments. Themes: CSRF attacker-model walkthrough is excellent pedagogy; three-party enumeration could note the browser as a fourth (mediator) participant; "Limitations" section is a model for honest engineering framing; Google Cloud Console steps likely to drift — flagged as a perennial verification target.

## Pass 2: Line Editing
- **Line ~40:** Minor comma/clause tightening.
  - Before: "which the server exchanges for an access token and uses to fetch the user's profile."
  - After: "which the server exchanges for an access token, then uses to fetch the user's profile."
  - Reason: The two verbs share a subject but feel rushed; a comma + "then" signals the sequence.
- **Line ~100:** More precise mitigation language.
  - Before: "The attacker cannot forge a valid state because they don't know what the server generated."
  - After: "The attacker cannot forge a valid state because they cannot predict what the server generated."
  - Reason: CSRF mitigation rests on unpredictability, not on secrecy of knowledge.
- **Line ~134:** Sharpen "simple" toward the later "Limitations" framing.
  - Before: "The cleanup loop on each call is a simple garbage collection strategy"
  - After: "The cleanup loop on each call is an ad hoc garbage-collection strategy"
  - Reason: Sets up the later upgrade path (Redis / signed tokens); "simple" is also on the style guide's suspect-word list.
- **Line ~235:** Soften a strong claim.
  - Before: "This is the most elegant solution."
  - After: "This is often the most elegant solution."
  - Reason: Trade-offs depend on context (stateless JWT can't be revoked without a blocklist); absolute claim overreaches.
- **Line ~237:** More concrete latency statement (optional).
  - Before: "Works but adds latency for something that should be fast."
  - After (suggested): "Works but adds a database round-trip on a hot path."

## Pass 3: Copy Editing
- **Line ~51–58:** Query — confirm current Google Cloud Console navigation (it changes often; the "APIs & Services > Credentials" path was accurate as of 2024).
- **Line ~78:** Query — confirm scopes `{"openid", "email", "profile"}` and `google.Endpoint` URL values are current.
- **Line ~111–115:** Query — confirm the `rand` import in source is `crypto/rand`, not `math/rand`; if the prose says "cryptographically random" but the code imports `math/rand`, the CSRF mitigation is broken.
- **Line ~164:** Query — the userinfo endpoint `https://www.googleapis.com/oauth2/v2/userinfo` is legacy; Google recommends the OIDC endpoint `https://openidconnect.googleapis.com/v1/userinfo`. Both work; consider noting the deprecation trajectory.
- **Line ~208:** Non-ASCII em dash inside Go source comment ("Existing user — issue token"). Confirm team tooling/editor accepts non-ASCII in source.
- **Line ~241:** "tradeoffs" — CMOS 7.85 leans "trade-offs"; both accepted. Ensure consistency with the rest of the book.
- **Line ~261:** Verified — RFC 6749 §10.12 is indeed "Cross-Site Request Forgery".

## Pass 4: Final Polish
- **References section:** No `[^N]` anchors in body text — functions as a bibliography.
- No typos, doubled words, or broken cross-refs found.
- "OAuth2" spelling consistent (vs. "OAuth 2.0" per RFC — consistency with the rest of the book is the priority; book uses "OAuth2").
