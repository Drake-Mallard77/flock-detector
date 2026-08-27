package httpapi

import (
	"encoding/json"
	"net/http"
)

type devLoginRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"` // "submitter" | "moderator" | "admin"
}

// handleDevLogin upserts a user by email and issues a JWT for the requested
// role, with no password/verification. Local development only.
func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	var req devLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	if req.Role == "" {
		req.Role = "submitter"
	}

	var userID string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO users (email, role)
		VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET role = EXCLUDED.role
		RETURNING id
	`, req.Email, req.Role).Scan(&userID)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not create/find user", err)
		return
	}

	token, err := s.issueToken(userID, req.Role)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not issue token", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token, "user_id": userID, "role": req.Role})
}
