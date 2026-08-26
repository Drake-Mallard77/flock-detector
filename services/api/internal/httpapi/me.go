package httpapi

import "net/http"

// handleMe returns the signed-in user's identity and role, read fresh from
// the database rather than from the token's claims.
//
// That re-read matters: tokens live for 24h, so a role revoked five minutes
// ago would still be asserted by an outstanding token. Anything the UI gates
// on must reflect current state, and the server re-checks on every
// role-gated action regardless.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	c, ok := r.Context().Value(claimsContextKey).(*claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	var email, role string
	err := s.db.QueryRow(r.Context(),
		`SELECT email, role FROM users WHERE id = $1`, c.Subject,
	).Scan(&email, &role)
	if err != nil {
		// The token referenced a user that no longer exists — treat it as
		// unauthenticated rather than a server error.
		writeError(w, http.StatusUnauthorized, "session is no longer valid")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"user_id": c.Subject,
		"email":   email,
		"role":    role,
	})
}
