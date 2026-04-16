# 4.1 Authentication Fundamentals

<!-- [STRUCTURAL] Strong opener: it orients the reader (two concepts), sets expectations (depth), and bridges prior knowledge (Spring Security analogy). Good pattern; keep this structure in future sections. -->
<!-- [LINE EDIT] "the Go implementations are just more explicit about what is happening under the hood" → The style guide says to cut "just". Suggest: "the Go implementations are simply more explicit about what happens under the hood" — but the guide also says to cut "simply". Best fix: "the Go implementations are more explicit about what happens under the hood". -->
Before writing any authentication code, you need to understand two core building blocks: how to store passwords safely and how to issue tokens that prove a user's identity. This section covers both topics in depth. If you have worked with Spring Security, many of these concepts will feel familiar -- the Go implementations are just more explicit about what is happening under the hood.

---

## Password Hashing with bcrypt

### Why Not SHA-256 or MD5?

<!-- [LINE EDIT] "If your first instinct is to hash passwords with SHA-256 or MD5, you are in good company -- but it is wrong." → Tighter: "If your first instinct is to hash passwords with SHA-256 or MD5, you have company — but you are wrong." The original "it is wrong" has an ambiguous referent (instinct? company? SHA-256?). -->
<!-- [COPY EDIT] "General-purpose hash functions" — correct use of hyphen for compound adjective before noun (CMOS 7.81). -->
If your first instinct is to hash passwords with SHA-256 or MD5, you are in good company -- but it is wrong. General-purpose hash functions have two properties that make them unsuitable for password storage:

<!-- [COPY EDIT] "A modern GPU can compute billions of SHA-256 hashes per second." — "billions" is fine in prose; numerals only required for technical measurements with units (CMOS 9.16). OK as-is. -->
1. **They are fast.** A modern GPU can compute billions of SHA-256 hashes per second. An attacker with a stolen database can brute-force short passwords in minutes.

<!-- [LINE EDIT] "They are deterministic without salting. The SHA-256 hash of "password123" is the same on every machine." — the word "deterministic" is the correct property, but "without salting" is oddly placed. Consider: "They are deterministic. Without salting, the SHA-256 hash of "password123" is identical on every machine." -->
<!-- [COPY EDIT] Straight quotes around "password123" — acceptable in fenced code or as a literal. In prose per CMOS 6.9 prefer curly quotes, but book toolchains typically auto-curl. Flag as style pass. -->
2. **They are deterministic without salting.** The SHA-256 hash of "password123" is the same on every machine. Attackers precompute tables of hash-to-password mappings (rainbow tables) and look up hashes instantly.

### Salts

A **salt** is a random value prepended to the password before hashing. Even if two users choose the same password, their salts differ, so their stored hashes differ. This defeats rainbow tables -- the attacker cannot precompute hashes for every possible salt.

<!-- [LINE EDIT] "Critically, the salt is stored alongside the hash. It is not secret. Its purpose is to make each hash unique, not to add secrecy." → Good, already concise. Keep. -->
<!-- [COPY EDIT] "Its purpose is to make each hash unique, not to add secrecy." — consider "Its purpose is to make each hash unique, not to provide secrecy." ("add secrecy" is slightly awkward collocation). -->
Critically, the salt is stored alongside the hash. It is not secret. Its purpose is to make each hash unique, not to add secrecy.

### bcrypt: Purpose-Built for Passwords

<!-- [STRUCTURAL] This subsection does strong work: definition + numeric intuition (cost factor = exponent, doubling per increment). Consider adding one sentence that ties back to the SHA-256 discussion: "This is precisely the property SHA-256 lacks." -->
<!-- [LINE EDIT] "it generates and embeds a salt automatically" → "and embeds a salt automatically" reads slightly redundant with "generates". Consider: "it embeds a random salt automatically." -->
<!-- [COPY EDIT] "2^10 = 1024" — prefer "2^10 = 1,024" per CMOS 9.56 (commas in integers ≥ 1,000); but for technical/computing context retaining "1024" is idiomatic and widely accepted. Leave as-is but flagging. -->
<!-- [COPY EDIT] "As hardware gets faster, you increase the cost factor -- this is what makes bcrypt "adaptive."" — period inside the closing quote per CMOS 6.9. Good. -->
bcrypt solves both problems. It is intentionally slow (configurable via a cost factor), and it generates and embeds a salt automatically. The cost factor is an exponent: cost 10 means 2^10 = 1024 rounds of the internal cipher. Increasing the cost by 1 doubles the time per hash. As hardware gets faster, you increase the cost factor -- this is what makes bcrypt "adaptive."

A bcrypt hash looks like this:

```
$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
```

<!-- [LINE EDIT] "The components are: algorithm identifier (`$2a$`), cost factor (`10`), 22-character salt, and 31-character hash -- all in one string." → Good, keep. Ends with useful summary "all in one string". -->
<!-- [COPY EDIT] Please verify: bcrypt hash components — is the 22-char block the salt and 31-char block the hash? Per the original bcrypt spec (Provos & Mazieres 1999), the encoded format is "$2a$<cost>$<22-char-base64-salt><31-char-base64-hash>" for a total trailing block of 53 chars. Confirm the "22 + 31" split is correct. -->
The components are: algorithm identifier (`$2a$`), cost factor (`10`), 22-character salt, and 31-character hash -- all in one string.

In Spring Security, you would use `BCryptPasswordEncoder`:

```java
BCryptPasswordEncoder encoder = new BCryptPasswordEncoder();
String hash = encoder.encode("plaintext");
boolean matches = encoder.matches("plaintext", hash);
```

In Go, the `golang.org/x/crypto/bcrypt` package provides the same functionality with two functions:

```go
import "golang.org/x/crypto/bcrypt"

// Hashing a password (registration)
hash, err := bcrypt.GenerateFromPassword([]byte("plaintext"), bcrypt.DefaultCost)
// hash is a []byte containing the full bcrypt string (salt + hash)

// Verifying a password (login)
err = bcrypt.CompareHashAndPassword(hash, []byte("plaintext"))
// err == nil means match; err != nil means mismatch
```

<!-- [LINE EDIT] "This takes roughly 100ms on modern hardware -- fast enough that users don't notice, slow enough that brute-force is impractical." → Good sentence. Keep. -->
<!-- [COPY EDIT] Please verify: "bcrypt.DefaultCost is 10. This takes roughly 100ms on modern hardware" — DefaultCost in golang.org/x/crypto/bcrypt is indeed 10 (per go.dev source, const DefaultCost = 10). Latency figure of ~100ms on "modern hardware" is an approximation; some sources cite 60–250ms depending on CPU. Consider softening: "typically around 100ms". -->
<!-- [COPY EDIT] "fast enough that users don't notice, slow enough that brute-force is impractical" — "brute-force" is hyphenated as an adjective (correct per CMOS 7.81); as a noun ("brute force attack") it would be open. Here it functions as the subject, so "brute forcing" or "a brute-force attack" would be more precise. Consider: "slow enough to make brute-force attacks impractical." -->
`bcrypt.DefaultCost` is 10. This takes roughly 100ms on modern hardware -- fast enough that users don't notice, slow enough that brute-force is impractical. Our auth service uses `DefaultCost` in the `Register` method:

```go
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
if err != nil {
    return "", nil, fmt.Errorf("failed to hash password: %w", err)
}
hashStr := string(hash)

user := &model.User{
    Email:        email,
    PasswordHash: &hashStr,
    Name:         name,
    Role:         "user",
}
```

<!-- [LINE EDIT] "Notice that `PasswordHash` is a `*string` (pointer), not a `string`. This is deliberate -- OAuth users have no password, so their `PasswordHash` is `nil`. We will revisit this design in section 4.3." → Good forward reference; keep. -->
<!-- [COPY EDIT] "section 4.3" — chapter-internal cross-reference; CMOS 8.180 says "section" lowercase when generic ("section 4.3"), but capitalized when part of a formal title ("Section 4.3"). Lowercase here is consistent with convention. -->
Notice that `PasswordHash` is a `*string` (pointer), not a `string`. This is deliberate -- OAuth users have no password, so their `PasswordHash` is `nil`. We will revisit this design in section 4.3.

---

## JSON Web Tokens (JWT)

### What Is a JWT?

<!-- [COPY EDIT] Heading case "What Is a JWT?" — CMOS 8.159 title case: capitalize first, last, and principal words; "Is" is a verb (capitalize), "a" is an article (lowercase). Current is correct. -->
A JSON Web Token is a compact, URL-safe string that carries a set of **claims** -- statements about a user. It has three parts, separated by dots:

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiI1NTBl...IjoxNjE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

<!-- [STRUCTURAL] Nicely paced: overall definition, then the concrete example, then the three-part breakdown. Good "tell, show, label" pattern. -->
Each part is base64url-encoded JSON:

<!-- [COPY EDIT] "base64url-encoded" — correct hyphenation for compound adjective before noun. Good. -->
1. **Header** -- the signing algorithm and token type:
   ```json
   { "alg": "HS256", "typ": "JWT" }
   ```

<!-- [COPY EDIT] "Standard claims include `sub` (subject / user ID), `iat` (issued at), and `exp` (expiration)." — serial comma correct. "iat" and "exp" are from RFC 7519 §4.1 — good. -->
<!-- [COPY EDIT] Please verify: "sub (subject / user ID)" — RFC 7519 §4.1.2 defines "sub" as "Subject of the JWT" with value being "a locally unique or globally unique identifier". The "user ID" gloss is an application-level convention, not RFC text. Fine as an explanation but worth confirming the pedagogical framing. -->
2. **Payload** -- the claims. Standard claims include `sub` (subject / user ID), `iat` (issued at), and `exp` (expiration). You can add custom claims like `role`:
   ```json
   { "sub": "550e8400-e29b-41d4-a716-446655440000", "role": "admin", "iat": 1616239022, "exp": 1616325422 }
   ```

<!-- [COPY EDIT] "HMAC-SHA256 of the header and payload, using a secret key" — "HMAC-SHA256" is correctly capitalized as an acronym-plus-hash-name. Good. -->
3. **Signature** -- HMAC-SHA256 of the header and payload, using a secret key:
   ```
   HMACSHA256(base64url(header) + "." + base64url(payload), secret)
   ```

<!-- [LINE EDIT] "Anyone can decode the header and payload (they are just base64), but only the server that knows the secret can produce a valid signature." → Drop "just" per style guide: "Anyone can decode the header and payload (they are plain base64), but only the server that knows the secret can produce a valid signature." -->
The signature ensures the token has not been tampered with. Anyone can decode the header and payload (they are just base64), but only the server that knows the secret can produce a valid signature.

### Our JWT Implementation

In `pkg/auth/jwt.go`, the `Claims` struct defines what we put in the token:

```go
type Claims struct {
    UserID uuid.UUID
    Role   string
    jwt.RegisteredClaims
}
```

<!-- [COPY EDIT] Please verify: "github.com/golang-jwt/jwt/v5" is the current maintained JWT module path. Confirmed active as of 2026. -->
<!-- [LINE EDIT] "it provides `Subject`, `IssuedAt`, `ExpiresAt`, and other standard fields" → serial comma present. Good. -->
`jwt.RegisteredClaims` is embedded from the `github.com/golang-jwt/jwt/v5` library -- it provides `Subject`, `IssuedAt`, `ExpiresAt`, and other standard fields. We add `UserID` and `Role` as custom claims.

Token generation creates claims, signs them with HMAC-SHA256, and returns the token string:

```go
func GenerateToken(userID uuid.UUID, role, secret string, expiry time.Duration) (string, error) {
    now := time.Now()
    claims := Claims{
        UserID: userID,
        Role:   role,
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   userID.String(),
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}
```

Validation parses the token, checks the signature and expiration, and returns the claims:

```go
func ValidateToken(tokenString, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(secret), nil
    })
    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }

    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token claims")
    }
    return claims, nil
}
```

<!-- [STRUCTURAL] Excellent security callout — naming the "alg: none" attack class with a footnote strengthens pedagogy. Consider one more sentence on *why* the HMAC check specifically defends: "An attacker who flips `alg` to `none` expects the library to skip signature verification entirely; by asserting `SigningMethodHMAC`, we reject any token that doesn't use our chosen algorithm family." -->
<!-- [LINE EDIT] "is a real vulnerability class" → "is a real vulnerability class, not a theoretical concern" — makes the stake explicit. -->
The callback function (`func(token *jwt.Token)`) is a **key provider** -- it returns the secret used to verify the signature. The signing method check (`*jwt.SigningMethodHMAC`) prevents an attacker from setting `alg: none` in the header to bypass verification. This is a real vulnerability class[^1].

---

## JWT vs. Session-Based Authentication

<!-- [STRUCTURAL] This comparison section is exactly the right length and uses the right devices (analogy, table, explicit winning criterion). The "Why JWT Wins for Microservices" sub-subsection is particularly strong. Keep structure. -->
<!-- [LINE EDIT] "If you have built web applications with Spring Security, you are used to `HttpSession`." → Active/tighter: "If you have built web applications with Spring Security, `HttpSession` will feel familiar." -->
If you have built web applications with Spring Security, you are used to `HttpSession`. A session works like this: the server creates a session ID, stores it in a server-side map (or Redis), and sends it to the client as a cookie. On each request, the server looks up the session ID to find the user.

<!-- [LINE EDIT] "JWTs flip this model. The token *contains* the user information. The server does not store anything -- it just validates the signature and reads the claims." → Drop "just": "it validates the signature and reads the claims." -->
JWTs flip this model. The token *contains* the user information. The server does not store anything -- it just validates the signature and reads the claims.

<!-- [COPY EDIT] Table column headings and cell content use sentence-case and fragment style consistently — good parallelism. -->
<!-- [COPY EDIT] "Stateless -- token is self-contained" — in tables, the "--" is fine; confirm en/em dash consistency with the rest of the book. -->
<!-- [COPY EDIT] "Any server can validate independently" — parallel with "Sticky sessions or shared store (Redis)". Good parallelism. -->
| Aspect | Session | JWT |
|---|---|---|
| State | Server must store session data | Stateless -- token is self-contained |
| Scaling | Sticky sessions or shared store (Redis) | Any server can validate independently |
| Revocation | Delete from session store | Cannot revoke until expiry (without a blocklist) |
| Size | Small cookie (~32 bytes) | Larger header (~300+ bytes) |
| Cross-service | Requires shared session store | Any service with the secret can validate |
| Analogy | Spring `HttpSession` | Spring Security + `JwtDecoder` |

### Why JWT Wins for Microservices

<!-- [LINE EDIT] "The critical advantage is the last row: **cross-service validation**." → Good. Keep. -->
<!-- [LINE EDIT] "In a monolith, a session store is fine -- there is only one server." → Technically a session store could be external even in a monolith; to be precise: "In a single-service application, a shared session store is easy — one process, one store." But the current framing is pedagogically cleaner. Leave. -->
<!-- [COPY EDIT] "With JWTs, you share the signing secret (or public key for asymmetric algorithms)" — parenthetical expands HS256 vs RS256 context without naming them; acceptable for tutorial flow. -->
The critical advantage is the last row: **cross-service validation**. In a monolith, a session store is fine -- there is only one server. In a microservices architecture, every service needs to verify the user's identity. With sessions, you need a centralized session store that all services query. With JWTs, you share the signing secret (or public key for asymmetric algorithms), and each service validates tokens independently. No network call, no shared database.

This is exactly our architecture. The Auth service issues tokens, and the Catalog service validates them using the same `pkg/auth` library and the same `JWT_SECRET` environment variable. Neither service needs to call the other to authenticate a request.

### The Revocation Problem

JWTs have one significant weakness: you cannot revoke them before they expire. If a user's account is compromised, you cannot invalidate their token. Common mitigations include:

<!-- [COPY EDIT] "our default is 24 hours" — numeric with time unit (CMOS 9.16 permits numerals for measurement). Good. -->
<!-- [COPY EDIT] "production systems often use 15 minutes with a separate refresh token" — numeric with unit. Good. -->
<!-- [LINE EDIT] "(check a Redis set on each validation -- but this reintroduces state)" → Parenthesized aside is fine; keep. -->
- **Short expiration times** (our default is 24 hours -- production systems often use 15 minutes with a separate refresh token)
- **Token blocklists** (check a Redis set on each validation -- but this reintroduces state)
- **Token versioning** (store a "token version" per user in the database; increment it on logout)

For this project, we use short-lived tokens without a blocklist. This is a reasonable tradeoff for a learning project and common in practice for internal microservice communication.

### When to Use Each

<!-- [STRUCTURAL] The three-bullet summary ("JWT / Sessions / Hybrid") is a good decision framework. Consider adding one-line example context to each (e.g., "— e.g., mobile client hitting a REST/gRPC gateway"). -->
<!-- [COPY EDIT] "APIs, microservices, mobile apps, any system where clients are not browsers or where multiple services need to authenticate independently." — missing serial comma before the trailing "any system" clause? Actually it functions as an appositive, not a list item. Leave. -->
- **JWT**: APIs, microservices, mobile apps, any system where clients are not browsers or where multiple services need to authenticate independently.
- **Sessions**: Traditional server-rendered web applications where the browser manages cookies and the application is a monolith.
- **Hybrid**: Some systems use sessions for browser clients (with CSRF protection) and JWTs for API/service-to-service calls.

---

## Summary

<!-- [STRUCTURAL] The summary bullets are well-chosen — each captures a durable takeaway, not a restatement. Keep the closing bridge sentence; it earns its place by naming the next section's artifact. -->
- **bcrypt** is the standard for password hashing. It is intentionally slow, embeds its own salt, and has an adjustable cost factor. Go's `golang.org/x/crypto/bcrypt` package wraps this in two functions.
- **JWTs** are self-contained tokens with a header, payload (claims), and HMAC signature. They enable stateless authentication -- no server-side session storage.
- For microservices, JWTs are the pragmatic choice because any service can validate them independently. Sessions require a shared store.
- JWTs cannot be revoked before expiry. Mitigations include short TTLs, refresh tokens, and blocklists.

In the next section, we build the Auth service that uses both of these primitives.

---

## References

<!-- [COPY EDIT] Footnote formatting — consistent "Source URL -- description" pattern. Good. -->
<!-- [COPY EDIT] "RFC 7519 -- JSON Web Token" — should use en dash for title/subtitle per CMOS 6.78, or em dash. Same "--" convention as elsewhere in the book. -->
<!-- [COPY EDIT] Please verify: "Provos & Mazieres, 1999" — the bcrypt paper is "A Future-Adaptable Password Scheme" by Niels Provos and David Mazières, published at USENIX 1999. The surname "Mazières" has an é; the source renders it as "Mazieres" (ASCII). Acceptable if the book uses ASCII-only references, otherwise prefer "Mazières". -->
[^1]: [JWT "alg: none" attack](https://auth0.com/blog/critical-vulnerabilities-in-json-web-token-libraries/) -- Auth0 blog post on critical JWT vulnerabilities including algorithm confusion attacks.
[^2]: [bcrypt paper (Provos & Mazieres, 1999)](https://www.usenix.org/legacy/events/usenix99/provos/provos.pdf) -- the original paper describing the bcrypt adaptive hashing function.
[^3]: [golang-jwt/jwt documentation](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) -- Go package documentation for the JWT library used in this project.
[^4]: [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) -- OWASP recommendations for secure password storage.
[^5]: [RFC 7519 -- JSON Web Token](https://datatracker.ietf.org/doc/html/rfc7519) -- the JWT specification.
