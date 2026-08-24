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

		token, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(s.cfg.JWTSecret), nil
		})
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
func (s *Server) requireRole(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := r.Context().Value(claimsContextKey).(*claims)
			if !ok {
				writeError(w, http.StatusUnauthorized, "missing claims")
				return
			}
			for _, role := range allowed {
				if c.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, http.StatusForbidden, "insufficient role")
		}))
	}
}
