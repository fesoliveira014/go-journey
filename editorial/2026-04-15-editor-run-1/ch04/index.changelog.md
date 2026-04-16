# Changelog: index.md

## Pass 1: Structural / Developmental
- 3 comments. Themes: chapter opener could frame *why* auth deserves its own chapter; diagram needs legend for dashed edges; "What You'll Learn" vs. "What You'll Build" overlap on a couple items.

## Pass 2: Line Editing
- **Line ~3:** Opening sentence is 40 words.
  - Before: "By the end, your library system will support user registration with bcrypt-hashed passwords, stateless JWT-based sessions, OAuth2 login with Google, and a shared interceptor that protects both the Auth and Catalog services."
  - After: "By the end, your library system will support user registration with bcrypt-hashed passwords, stateless JWT-based sessions, and OAuth2 login with Google. A shared interceptor will protect both the Auth and Catalog services."
  - Reason: Split at 40-word threshold for readability.
- **Line ~36:** Drop throwaway "that".
  - Before: "it lives outside both services so that any microservice can import it"
  - After: "it lives outside both services so any microservice can import it"
  - Reason: Filler "that" removable without loss.
- **Line ~49:** Tighten passive/soft phrasing.
  - Before: "(the Catalog service and Docker Compose stack should build and run)"
  - After: "(the Catalog service and Docker Compose stack build and run cleanly)"
  - Reason: Active, crisper.

## Pass 3: Copy Editing
- **Line ~36, 44, 62–65:** Flagged the "--" dash convention used throughout the chapter as a query (CMOS 6.85). If the static site generator does not convert these to em dashes, replace with Unicode em dashes without spaces.
- **Line ~56:** Serial comma (CMOS 6.19) — "Register, Login, ValidateToken, GetUser, InitOAuth2, and CompleteOAuth2".
- **Line ~56:** "six gRPC RPCs" → consider "six RPCs" to avoid redundancy (RPC already in gRPC).

## Pass 4: Final Polish
- No typos or doubled words found.
- Cross-references to sibling files use correct relative paths.
