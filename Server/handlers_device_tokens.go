package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
)

type registerDeviceTokenRequest struct {
	Token string `json:"token"`
}

// RegisterDeviceTokenRoutes attaches device-token endpoints under /api.
func RegisterDeviceTokenRoutes(r chi.Router) {
	r.Post("/device-tokens/register", handleRegisterDeviceToken)
}

func handleRegisterDeviceToken(w http.ResponseWriter, r *http.Request) {
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	var req registerDeviceTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}
	if len(token) < 16 || len(token) > 256 {
		http.Error(w, "token must be between 16 and 256 characters", http.StatusBadRequest)
		return
	}

	// SECURITY: Verify token has mixed character types (not just repetition).
	// This encourages use of randomized tokens from hardware.
	if !isTokenSufficientlyRandom(token) {
		http.Error(w, "token must contain a mix of character types (uppercase, lowercase, digits)", http.StatusBadRequest)
		return
	}

	tokenHash := hashDeviceToken(token)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const insertQ = `
		INSERT INTO device_tokens (user_id, token_hash)
		VALUES ($1, $2)
		ON CONFLICT (token_hash) DO NOTHING;
	`

	tag, err := dbPool.Exec(ctx, insertQ, userID, tokenHash)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if tag.RowsAffected() == 0 {
		const ownerQ = `
			SELECT user_id
			FROM device_tokens
			WHERE token_hash = $1
			LIMIT 1;
		`

		var ownerID int64
		if err := dbPool.QueryRow(ctx, ownerQ, tokenHash).Scan(&ownerID); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}

		if ownerID != userID {
			http.Error(w, "token is already linked to another user", http.StatusConflict)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"created": false,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"created": true,
	})
}

// isTokenSufficientlyRandom checks if token has mixed character types.
// This encourages use of real random tokens instead of simple repetition.
// Hardware devices should generate cryptographically random tokens.
func isTokenSufficientlyRandom(token string) bool {
	hasUpper := false
	hasLower := false
	hasDigit := false

	for _, ch := range token {
		if unicode.IsUpper(ch) {
			hasUpper = true
		}
		if unicode.IsLower(ch) {
			hasLower = true
		}
		if unicode.IsDigit(ch) {
			hasDigit = true
		}
	}

	// Require at least 2 of the 3 character types
	count := 0
	if hasUpper {
		count++
	}
	if hasLower {
		count++
	}
	if hasDigit {
		count++
	}

	return count >= 2
}
