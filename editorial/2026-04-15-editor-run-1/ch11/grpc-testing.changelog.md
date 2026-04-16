# Changelog: grpc-testing.md

## Pass 1: Structural / Developmental
- 2 STRUCTURAL comments. Themes:
  - 11.3 uses `grpc.NewClient` with `passthrough:///bufconn`; 11.5 uses `grpc.DialContext` with `"bufnet"`. Same book should pick one API. `grpc.NewClient` is current (introduced grpc-go 1.64); align 11.5 to it.
  - No coverage of streaming RPCs; acceptable for project scope but consider a one-line "out of scope" note for thoroughness.

## Pass 2: Line Editing
- No single sentence crossed the 40-word threshold with problematic structure. Most long sentences read cleanly.
- Minor polish opportunities noted inline.

## Pass 3: Copy Editing
- **Line ~17:** "serialises" → "serializes" (US spelling).
- **Line ~19:** "serialises" / "deserialises" → "serializes" / "deserializes" (US).
- **Line ~21:** "serialised" → "serialized" (US).
- **Line ~24:** "authorisation" → "authorization" (US).
- **Line ~44:** "behaviour" → "behavior" (US).
- **Line ~46:** "host name" → "hostname" (CMOS 7.85 closed compound).
- **Line ~131:** Heading "Testing interceptor behaviour" → "Testing interceptor behavior" (US).
- **Line ~189:** Cross-reference "Section 11.2" — consistent form; good.
- **Line ~211:** Heading "Combining bufconn with testcontainers" → "Combining bufconn with Testcontainers" (capitalize product in prose).
- **Line ~301:** ASCII arrow vs Unicode arrow in "register -> login -> validate-token" — pick one style and keep consistent across chapter. Recommend Unicode `→` for prose and plain `->` reserved for code comments.
- **Line ~363:** Table row "bufconn + testcontainers" → "bufconn + Testcontainers".
- **Line ~366:** "error code mapping" → "error-code mapping" (CMOS 7.81 compound adjective).
- **Line ~75:** Please verify: `grpc.NewClient` target string `"passthrough:///bufconn"` — current grpc-go convention is `"passthrough:"` prefix (no triple slash) with scheme/endpoint but both forms often accepted. Confirm.
- **Line ~113:** Please verify: that `grpc.NewClient` is the currently recommended API for test wiring over bufconn per grpc-go docs.
- **Line ~370:** Please verify: footnote URLs still resolve.

## Pass 4: Final Polish
- **Line ~166:** `token, _ := pkgauth.GenerateToken(...)` discards error on a security-sensitive path. Replace with explicit `if err != nil { t.Fatalf(...) }` for pedagogical clarity (consistent with the 11.5 version, which does check).
- **Line ~228:** Same `token, _` pattern; same recommendation.
- Heading "Testing interceptor behaviour" — spelling flagged in Pass 3.
