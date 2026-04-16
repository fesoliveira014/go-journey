# Changelog: reservation-service.md

## Pass 1: Structural / Developmental
- 10 comments. Themes:
  - **Pervasive "availability check" vs. "guarded decrement" framing problem.** The file opens by calling the catalog gRPC call a "cross-service read" / "check book availability," then the TOCTOU section rightly corrects this to a "guarded decrement." The opener and multiple later mentions contradict the correct framing established midway through. Normalize the framing from the first paragraph onward.
  - **Incorrect claim in TOCTOU step 3.** The text says the reaper is a "backstop" for a failed compensation, asserting "an unpaired decrement will eventually be reconciled when other reservations expire and the numbers converge." That is not true — the reaper only expires existing rows; an orphan decrement with no paired row will never be healed by the reaper. This is a real gap, not a backstop, and the prose should say so.
  - **Elided ReservationEvent payload** in the CreateReservation snippet. Readers have to jump back to 7.1 to see what is published. Inline the struct literal (or add a comment showing the fields).
  - **Security framing.** The ownership check section could name the vulnerability class (IDOR / Insecure Direct Object Reference) for readers who will see the term in OWASP.
  - Suggested reference additions: Saga pattern (Richardson), Postgres row-level locking docs. Inline TOCTOU Wikipedia link already present.
  - Exercises are strong. Consider adding a "repo.Create fails after decrement" test case to exercise 2.

## Pass 2: Line Editing
- **Line ~90:** "currently active" redundancy.
  - Before: "it counts the user's currently active reservations."
  - After: "it counts the user's active reservations."
  - Reason: "Active" already implies "currently."
- **Line ~116:** Tail filler.
  - Before: "for learning purposes, this is enough to demonstrate the pattern."
  - After: "for learning purposes, this is enough."
  - Reason: Cuts "to demonstrate the pattern."
- **Line ~121:** Tighten.
  - Before: "The service layer is where the interesting logic lives. Let us look at its dependencies:"
  - After: "The service layer is where the domain logic lives. Its dependencies:"
  - Reason: Replaces subjective "interesting" with "domain"; drops "Let us look at" preamble.
- **Line ~145:** Tighten the pre-pointer to TOCTOU.
  - Before: "The interesting part is the order of operations — we decrement availability **before** creating the reservation row, not after. The next section (_The TOCTOU trap_) explains why."
  - After: "The ordering is the interesting bit — we decrement availability **before** inserting the reservation row. The next section, *The TOCTOU trap*, explains why."
  - Reason: Removes parenthetical clutter, parallel construction.
- **Line ~218:** Combine two short sentences.
  - Before: "It is also **wrong** under concurrency. This is a textbook [Time-Of-Check-to-Time-Of-Use][toctou] bug:"
  - After: "It is also **wrong** under concurrency — this is a textbook time-of-check-to-time-of-use (TOCTOU) bug:"
  - Reason: Joins; spells out acronym on first use.
- **Line ~294:** Informal "observe".
  - Before: "users never observe their own overdue reservations still listed as active"
  - After: "users never see their own overdue reservations still listed as active"
  - Reason: Matches the register of surrounding prose.
- **Line ~336:** Drop leading "And".
  - Before: "And because the reaper runs with a background context that has no user attached, the helper falls back to the reservation's own UserID when publishing the event."
  - After: "Because the reaper's background context has no user attached, the helper falls back to the reservation's own UserID when publishing the event."
  - Reason: Cleaner; removes the coordinator "And" after a sentence break.
- **Line ~431:** Fix overstatement.
  - Before: "Three lines to wire the entire application:"
  - After: "Three lines to wire the domain stack:"
  - Reason: main.go wires more than these three lines (DB, listener, clients, etc.).

## Pass 3: Copy Editing
- **Global (dash style):** Mixed em dash and spaced en dash (` -- `). Normalize to one per house style. (CMOS 6.85)
- **Global (spelling variants):** "serialises", "cancelled" (UK). If US house style, change to "serializes", "canceled". (CMOS 7.4)
- **Line ~35:** ASCII arrow `->` in prose — consider Unicode `→` to match the index architecture diagram. (Style consistency.)
- **Line ~206:** Heading "The TOCTOU trap (and why we decrement first)" — mixed case. Normalize to title case: "The TOCTOU Trap (and Why We Decrement First)". (CMOS 8.159)
- **Line ~218:** "Time-Of-Check-to-Time-Of-Use" — conventional form is "time-of-check-to-time-of-use" (all lowercase with hyphens). (Domain/Wikipedia convention.)
- **Line ~224:** Correctness — rewrite step 3 of the TOCTOU fix. The claim that the reaper reconciles an unpaired decrement is incorrect; the counter will stay off by one until operator reconciliation or a separate job fixes it.
- **Line ~224:** "DB down" → "database down" (avoid informal abbreviation in a reference text).
- **Line ~332:** "env var" → "environment variable" (spelled out in prose).
- **Line ~332:** "5 minutes" — numerals acceptable for a technical interval, but note the convention decision. (CMOS 9.2.)
- **Line ~414:** Please verify — HTTP status code mappings. gRPC→HTTP spec maps `FailedPrecondition` to 400, not 412, in the generic mapping. The book's gateway (7.4) uses 412 (`StatusPreconditionFailed`). This is a book-internal consistency statement that is correct *for this codebase*, but the phrasing "it maps them to HTTP status codes" should acknowledge the codebase-specific choice rather than implying it is the gRPC standard.
- **Line ~422:** "main.go function" — `main.go` is the file; the function is `main`. Reword.
- **Line ~438:** "since it needs to check availability" → "since it needs to reserve copies through the catalog" (framing fix).
- **Line ~443:** "synchronous catalog lookup" → "synchronous catalog call" (framing fix).
- **Line ~459:** "calling catalog for availability checks" → "reserving copies against the catalog" (framing fix).
- **References:** ` -- ` in footnote entries — standardize to em dash or period.

## Pass 4: Final Polish
- **Line ~14:** Code-comment file paths (e.g., `services/reservation/internal/model/model.go`) — verify against current repo layout.
- **Line ~124:** Same file-path check for `service.go`.
- **Line ~207:** Verify the "earlier version" code is representative (i.e., not a fabrication but matches the actual commit history or a realistic prior design).
- **Line ~277:** Consider bolding "IDOR" for OWASP cross-reference.
- **Line ~468:** Exercise 2 — add a fourth case ("catalog decrement succeeds, repo.Create fails; verify compensation") to exercise the most subtle path.
