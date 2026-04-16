# 4.1 Authentication Fundamentals

Before writing any authentication code, you need to understand two core building blocks: how to store passwords safely and how to issue tokens that prove a user's identity. This section covers both topics in depth. If you have worked with Spring Security, many of these concepts will feel familiar—the Go implementations are more explicit about the underlying mechanics.

---

## Password Hashing with bcrypt

### Why Not SHA-256 or MD5?

If your first instinct is to hash passwords with SHA-256 or MD5, you have company—but you are wrong. General-purpose hash functions have two properties that make them unsuitable for password storage:

1. **They are fast.** A modern GPU can compute billions of SHA-256 hashes per second. An attacker with a stolen database can brute-force short passwords in minutes.

2. **They are deterministic.** Without salting, the SHA-256 hash of "password123" is identical on every machine. Attackers precompute tables of hash-to-password mappings (rainbow tables) and look up hashes instantly.

### Salts

A **salt** is a random value prepended to the password before hashing. Even if two users choose the same password, their salts differ, so their stored hashes differ. This defeats rainbow tables—the attacker cannot precompute hashes for every possible salt.

Critically, the salt is stored alongside the hash. It is not secret. Its purpose is to make each hash unique, not to add secrecy.

### bcrypt: Purpose-Built for Passwords

bcrypt solves both problems. It is intentionally slow (configurable via a cost factor), and it embeds a random salt automatically. The cost factor is an exponent: cost 10 means 2^10 = 1,024 iterations of the key-expansion phase. Increasing the cost by 1 doubles the time per hash. As hardware gets faster, you increase the cost factor—this is what makes bcrypt "adaptive."

A bcrypt hash looks like this:

```
$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
```

The components are: algorithm identifier (`$2a$`), cost factor (`10`), 22-character salt, and 31-character hash—all in one string.

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

`bcrypt.DefaultCost` is 10. This takes roughly 100ms on modern hardware—fast enough that users don't notice, slow enough to make brute-force attacks impractical. Our Auth Service uses `DefaultCost` in the `Register` method:

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

Notice that `PasswordHash` is a `*string` (pointer), not a `string`. This is deliberate—OAuth users have no password, so their `PasswordHash` is `nil`. We will revisit this design in section 4.3.

---

## JSON Web Tokens (JWT)

### What Is a JWT?

A JSON Web Token is a compact, URL-safe string that carries a set of **claims**—statements about a user. It has three parts, separated by dots:

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiI1NTBl...IjoxNjE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

Each part is base64url-encoded JSON:

1. **Header**—the signing algorithm and token type:
   ```json
   { "alg": "HS256", "typ": "JWT" }
   ```

2. **Payload**—the claims. Standard claims include `sub` (subject / user ID), `iat` (issued at), and `exp` (expiration). You can add custom claims like `role`:
   ```json
   { "sub": "550e8400-e29b-41d4-a716-446655440000", "role": "admin", "iat": 1616239022, "exp": 1616325422 }
   ```

3. **Signature**—HMAC-SHA256 of the header and payload, using a secret key:
   ```
   HMACSHA256(base64url(header) + "." + base64url(payload), secret)
   ```

The signature ensures the token has not been tampered with. Anyone can decode the header and payload (they are plain base64), but only the server that knows the secret can produce a valid signature.

### Our JWT Implementation

In `pkg/auth/jwt.go`, the `Claims` struct defines what we put in the token:

```go
type Claims struct {
    UserID uuid.UUID
    Role   string
    jwt.RegisteredClaims
}
```

`jwt.RegisteredClaims` is embedded from the `github.com/golang-jwt/jwt/v5` library—it provides `Subject`, `IssuedAt`, `ExpiresAt`, and other standard fields. We add `UserID` and `Role` as custom claims.

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

The callback function (`func(token *jwt.Token)`) is a **key provider**—it returns the secret used to verify the signature. The signing method check (`*jwt.SigningMethodHMAC`) prevents an attacker from setting `alg: none` in the header to bypass verification. This is a real vulnerability class, not a theoretical concern.[^1]

---

## JWT vs. Session-Based Authentication

If you have built web applications with Spring Security, `HttpSession` will feel familiar. A session works like this: the server creates a session ID, stores it in a server-side map (or Redis), and sends it to the client as a cookie. On each request, the server looks up the session ID to find the user.

JWTs flip this model. The token *contains* the user information. The server does not store anything—it validates the signature and reads the claims.

| Aspect | Session | JWT |
|---|---|---|
| State | Server must store session data | Stateless—token is self-contained |
| Scaling | Sticky sessions or shared store (Redis) | Any server can validate independently |
| Revocation | Delete from session store | Cannot revoke until expiry (without a blocklist) |
| Size | Small cookie (~32 bytes) | Larger header (~300+ bytes) |
| Cross-service | Requires shared session store | Any service with the secret can validate |
| Analogy | Spring `HttpSession` | Spring Security + `JwtDecoder` |

### Why JWT Wins for Microservices

The critical advantage is the last row: **cross-service validation**. In a monolith, a session store is fine—there is only one server. In a microservices architecture, every service needs to verify the user's identity. With sessions, you need a centralized session store that all services query. With JWTs, you share the signing secret (or public key for asymmetric algorithms), and each service validates tokens independently. No network call, no shared database.

This is exactly our architecture. The Auth service issues tokens, and the Catalog service validates them using the same `pkg/auth` library and the same `JWT_SECRET` environment variable. Neither service needs to call the other to authenticate a request.

### The Revocation Problem

JWTs have one significant weakness: You cannot revoke them before they expire. If a user's account is compromised, you cannot invalidate their token. Common mitigations include:

- **Short expiration times** (our default is 24 hours—production systems often use 15 minutes with a separate refresh token)
- **Token blocklists** (check a Redis set on each validation—but this reintroduces state)
- **Token versioning** (store a "token version" per user in the database; increment it on logout)

For this project, we use short-lived tokens without a blocklist. This is a reasonable trade-off for a learning project and common in practice for internal microservice communication.

### When to Use Each

- **JWT**: APIs, microservices, mobile apps, or any system where clients are not browsers or where multiple services need to authenticate independently.
- **Sessions**: Traditional server-rendered web applications where the browser manages cookies and the application is a monolith.
- **Hybrid**: Some systems use sessions for browser clients (with CSRF protection) and JWTs for API/service-to-service calls.

---

## Summary

- **bcrypt** is the standard for password hashing. It is intentionally slow, embeds its own salt, and has an adjustable cost factor. Go's `golang.org/x/crypto/bcrypt` package wraps this in two functions.
- **JWTs** are self-contained tokens with a header, payload (claims), and HMAC signature. They enable stateless authentication—no server-side session storage.
- For microservices, JWTs are the pragmatic choice because any service can validate them independently. Sessions require a shared store.
- JWTs cannot be revoked before expiry. Mitigations include short TTLs, refresh tokens, and blocklists.

In the next section, we build the Auth service that uses both of these primitives.

---

## References

[^1]: [JWT "alg: none" attack](https://auth0.com/blog/critical-vulnerabilities-in-json-web-token-libraries/)—Auth0 blog post on critical JWT vulnerabilities including algorithm confusion attacks.
[^2]: [bcrypt paper (Provos & Mazieres, 1999)](https://www.usenix.org/legacy/events/usenix99/provos/provos.pdf)—the original paper describing the bcrypt adaptive hashing function.
[^3]: [golang-jwt/jwt documentation](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)—Go package documentation for the JWT library used in this project.
[^4]: [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)—OWASP recommendations for secure password storage.
[^5]: [RFC 7519—JSON Web Token](https://datatracker.ietf.org/doc/html/rfc7519)—the JWT specification.
