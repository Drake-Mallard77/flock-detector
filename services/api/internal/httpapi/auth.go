package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsContextKey contextKey = "claims"

type claims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

func (s *Server) issueToken(userID, role string) (string, error) {
	c := claims{
		Subject: userID,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

// requireAuth parses a Bearer token and attaches its claims to the request
// context. It does not check role; use requireRole for that.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		tokenStr, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || tokenStr == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		// WithValidMethods pins verification to HS256: without it, a party
		// that doesn't know JWTSecret could still craft a token using a
		// different algorithm (e.g. "none") that some parsers accept.
		token, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(s.cfg.JWTSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		c, ok := token.Claims.(*claims)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, c)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireRole wraps requireAuth and additionally checks the caller's role is
// one of the allowed roles (e.g. "moderator", "admin").
//
// The role is re-read from the database on every request rather than trusted
// from the token's claims. Tokens live for 24h, so a role revoked after a
// token was issued would otherwise keep working for the rest of that window —
// on a site where a moderator session can rewrite the public record, losing
// the ability to revoke access promptly is not acceptable. The extra query
// only runs on role-gated endpoints, which are moderation actions rather
// than hot paths.
func (s *Server) requireRole(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := r.Context().Value(claimsContextKey).(*claims)
			if !ok {
				writeError(w, http.StatusUnauthorized, "missing claims")
				return
			}

			var currentRole string
			if err := s.db.QueryRow(r.Context(),
				`SELECT role FROM users WHERE id = $1`, c.Subject,
			).Scan(&currentRole); err != nil {
				writeError(w, http.StatusUnauthorized, "session is no longer valid")
				return
			}

			for _, role := range allowed {
				if currentRole == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, http.StatusForbidden, "insufficient role")
		}))
	}
}
