package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var ErrDeviceTokenInvalid = errors.New("device token invalid")

type repRequest struct {
	Reps   int    `json:"reps"`
	Source string `json:"source"`
	Scope  string `json:"scope"`
}

// RegisterRepRoutes attaches rep ingestion endpoints under /api.
func RegisterRepRoutes(r chi.Router) {
	r.Post("/reps", handleReps)
}

// handleReps accepts device or session-authenticated rep submissions.
func handleReps(w http.ResponseWriter, r *http.Request) {
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
		token := strings.TrimSpace(r.Header.Get("X-Device-Token"))
		if token == "" {
			http.Error(w, "missing device token", http.StatusUnauthorized)
			return
		}

		var err error
		userID, err = userIDFromDeviceToken(r.Context(), token)
		if err != nil {
			http.Error(w, "invalid device token", http.StatusUnauthorized)
			return
		}
	}

	var req repRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Reps <= 0 || req.Reps > 1000 {
		http.Error(w, "invalid reps", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const q = `
		INSERT INTO rep_sessions (user_id, reps, scope, source)
		VALUES ($1, $2, $3, $4);
	`

	if _, err := dbPool.Exec(ctx, q, userID, req.Reps, req.Scope, req.Source); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func userIDFromDeviceToken(ctx context.Context, token string) (int64, error) {
	hash := hashDeviceToken(token)

	const q = `
		SELECT user_id
		FROM device_tokens
		WHERE token_hash = $1;
	`

	var userID int64
	err := dbPool.QueryRow(ctx, q, hash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrDeviceTokenInvalid
		}
		return 0, err
	}

	return userID, nil
}

func hashDeviceToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
