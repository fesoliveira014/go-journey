package middleware

import (
	"net/http"

	pkgauth "github.com/fesoliveira014/library-system/pkg/auth"
)

func Auth(next http.Handler, jwtSecret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := pkgauth.ValidateToken(cookie.Value, jwtSecret)
		if err != nil {
			// Invalid/expired token — continue as anonymous
			next.ServeHTTP(w, r)
			return
		}
		ctx := pkgauth.ContextWithUser(r.Context(), claims.UserID, claims.Role)
		ctx = pkgauth.ContextWithToken(ctx, cookie.Value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
