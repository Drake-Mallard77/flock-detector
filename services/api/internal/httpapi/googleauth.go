package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/api/idtoken"
)

type googleLoginRequest struct {
	// The ID token (JWT) issued by Google Identity Services to the browser.
	Credential string `json:"credential"`
}

// handleGoogleLogin exchanges a Google ID token for a FlockWatch session
// token.
//
// Two security properties this must preserve:
//
//  1. The Google token is *validated*, not merely decoded — idtoken.Validate
//     checks the signature against Google's rotating JWKS, plus issuer,
//     expiry, and that the audience is our own client ID. Skipping the
//     audience check would let a token minted for any other Google app be
//     replayed here.
//
//  2. The role comes from our own database, never from the token or the
//     request body. Signing in with Google proves an email address and
//     nothing else; moderator access is granted out-of-band (see
//     cmd/grant-role). A first-time signer-in gets 'submitter', which can
//     do no more than an anonymous visitor.
func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GoogleClientID == "" {
		// Fail closed: without a client ID there is no audience to check,
		// so any Google-issued token would otherwise be accepted.
		slog.WarnContext(r.Context(), "Google sign-in attempted but GOOGLE_CLIENT_ID is not set")
		writeError(w, http.StatusServiceUnavailable, "Google sign-in is not configured on this server")
		return
	}

	var req googleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Credential == "" {
		writeError(w, http.StatusBadRequest, "credential is required")
		return
	}

	payload, err := idtoken.Validate(r.Context(), req.Credential, s.cfg.GoogleClientID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid Google credential")
		return
	}

	email, verified, err := emailFromPayload(payload)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if !verified {
		// An unverified address can be attacker-controlled, which would
		// otherwise let someone claim a moderator's email.
		writeError(w, http.StatusForbidden, "your Google account's email address is not verified")
		return
	}

	// Upsert on email, but deliberately do NOT touch role here: an existing
	// moderator signing in again must keep their role, and a new user must
	// not be able to pick one.
	var userID, role string
	err = s.db.QueryRow(r.Context(), `
		INSERT INTO users (email, role)
		VALUES ($1, 'submitter')
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id, role
	`, email).Scan(&userID, &role)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not sign you in", err)
		return
	}

	token, err := s.issueToken(userID, role)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not issue token", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
		"email": email,
		"role":  role,
	})
}

func emailFromPayload(payload *idtoken.Payload) (email string, verified bool, err error) {
	raw, ok := payload.Claims["email"].(string)
	if !ok || raw == "" {
		return "", false, errors.New("Google account did not provide an email address")
	}

	// email_verified arrives as a bool from Google, but tolerate the string
	// form some OIDC providers use rather than silently treating it as false.
	switch v := payload.Claims["email_verified"].(type) {
	case bool:
		verified = v
	case string:
		verified = v == "true"
	}

	return strings.ToLower(strings.TrimSpace(raw)), verified, nil
}
