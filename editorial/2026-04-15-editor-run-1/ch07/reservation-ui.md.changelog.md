# Changelog: reservation-ui.md

## Pass 1: Structural / Developmental
- 8 comments. Themes:
  - **Critical inconsistency with 7.2: the Eventual Consistency section.** The motivating example describes a window where the *create* operation has not yet decremented availability. That only holds if the decrement happens asynchronously via Kafka. Section 7.2 establishes the decrement is *synchronous* (TOCTOU fix). The section's scenario must be reanchored to the async paths (return/expire), where eventual consistency actually applies.
  - **Testing-the-Full-Flow steps are mis-annotated.** Step 5 (verify availability decreased after reserve) should be immediate; step 7 (verify availability increased after return) is the async path. The caveat "wait a second and refresh" applies to step 7, not step 5.
  - **Flash cookie hardening.** The flash cookie is unsigned and has neither `Secure` nor `SameSite` set. For a learning project this is acceptable, but worth a one-sentence caveat. CSRF is addressed in exercise 5.
  - **Index.md cross-reference says "Docker setup"** but this section has only a sentence-length mention. Either add a mini-walkthrough or update index.md. (Flagged in index.md changelog too.)
  - Go reference-time mnemonic (1, 2, 3, 4, 5, 6, 7) would aid retention.
  - Exercise 2: the N+1 prompt could hint at a `BatchGetBooks` RPC shape.

## Pass 2: Line Editing
- **Line ~3:** Cut "just".
  - Before: "No new architectural concepts -- just applying the patterns we already know to a new feature."
  - After: "No new architectural concepts — we apply the patterns we already know to a new feature."
  - Reason: Filler.
- **Line ~20:** Clarify "forms".
  - Before: "There are no GET forms for reserve or return"
  - After: "There are no GET endpoints for reserve or return"
  - Reason: "Forms" ambiguous against HTML `<form>`.
- **Line ~20:** Pluralize "URL".
  - Before: "not pages with their own URL"
  - After: "not pages with their own URLs"
  - Reason: Plural agreement with "pages".
- **Line ~224:** Cut "just".
  - Before: "The gateway just relays."
  - After: "The gateway relays."
  - Reason: Filler.
- **Line ~277:** Join short sentences.
  - Before: "We do none of these. The simplest approach is often the right one: accept the brief inconsistency and let the system converge naturally."
  - After: "We do none of these — the simplest approach is often the right one: accept the brief inconsistency and let the system converge."
  - Reason: Tightens transition; removes "naturally" (redundant with "converge").

## Pass 3: Copy Editing
- **Global (dash style):** ` -- ` throughout; normalize per chapter-wide decision. (CMOS 6.85)
- **Line ~3:** "BFF" — expand on first use: "Backend for Frontend (BFF)". (CMOS 10.3)
- **Line ~14:** Note Go 1.22+ method-pattern syntax on first appearance (or cross-ref to Chapter 5 if already explained there).
- **Line ~47:** "10+ clients" — acceptable in technical prose; formal is "ten or more clients". (CMOS 9.2)
- **Line ~108:** "MaxAge: 10 means the cookie expires after 10 seconds" — clarify the unit: "MaxAge: 10 (seconds) means…".
- **Line ~108 block:** Cookie hardening — consider noting absence of `Secure` and `SameSite`; unsigned value.
- **Line ~192:** Completing Go reference time — add mnemonic `Mon Jan 2 15:04:05 MST 2006` (1/2/3/4/5/6/7).
- **Line ~261:** Capitalization check — "The reservation is created (instant)" is followed by Kafka steps; verify the chain against the actual implementation after reanchoring this section.
- **Line ~308:** "gorilla/csrf" — please verify current maintenance status and consider also naming `github.com/justinas/nosurf`.
- **Line ~310:** "stdlib" → "standard library" in formal prose.
- **References:** " -- " in footnote entries — standardize. Please verify MDN URL path (`/Web/HTTP/Reference/Status/303` vs. the older `/Web/HTTP/Status/303`).

## Pass 4: Final Polish
- **Line ~13:** Code-comment file path `services/gateway/cmd/main.go` — verify.
- **Line ~70:** `services/gateway/internal/handler/reservation.go` — verify.
- **Line ~56:** `services/gateway/templates/book.html` — verify.
- **Line ~147:** `services/gateway/templates/reservations.html` — verify.
- **Line ~286:** Mis-annotated verification step: rewrite step 5/7 commentary to match the synchronous-decrement / async-increment reality from 7.2.
- **Line ~296:** "check the catalog service logs for consumer errors" — attach a pointer example (e.g., "docker compose logs -f catalog | grep 'handle event'") to make the troubleshooting concrete.
