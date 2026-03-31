package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type profileResponse struct {
	Username     string `json:"username"`
	CreatedAt    string `json:"createdAt"`
	TotalReps    int64  `json:"totalReps"`
	StreakDays   int    `json:"streakDays"`
	IsFounder    bool   `json:"isFounder"`
	FriendsCount int    `json:"friendsCount"`
	IsSelf       bool   `json:"isSelf"`
}

// RegisterProfileRoutes attaches profile endpoints under /api.
func RegisterProfileRoutes(r chi.Router) {
	r.Get("/profiles/{username}", handleGetProfile)
	r.Delete("/profiles/me", handleDeleteOwnProfile)
}

func handleGetProfile(w http.ResponseWriter, r *http.Request) {
	viewerID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if viewerID == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		http.Error(w, "missing username", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const q = `
		WITH target_user AS (
			SELECT u.id, u.username, u.created_at
			FROM users u
			WHERE u.username = $1
		),
		daily_activity AS (
			SELECT DISTINCT
				rs.user_id,
				(rs.created_at AT TIME ZONE 'UTC')::date AS activity_day
			FROM rep_sessions rs
		),
		streaks AS (
			SELECT
				da.user_id,
				COUNT(*) FILTER (
					WHERE da.activity_day = (d.utc_today - (da.rn - 1))
				)::int AS streak_days
			FROM (
				SELECT
					user_id,
					activity_day,
					ROW_NUMBER() OVER (
						PARTITION BY user_id
						ORDER BY activity_day DESC
					)::int AS rn
				FROM daily_activity
			) da
			CROSS JOIN (
				SELECT (NOW() AT TIME ZONE 'UTC')::date AS utc_today
			) d
			GROUP BY da.user_id
		),
		founder_candidates AS (
			SELECT
				dt.user_id,
				MIN(dt.created_at) AS first_registered_at
			FROM device_tokens dt
			GROUP BY dt.user_id
		),
		founders AS (
			SELECT fc.user_id
			FROM founder_candidates fc
			ORDER BY fc.first_registered_at ASC, fc.user_id ASC
			LIMIT 50
		),
		friend_counts AS (
			SELECT f.user_id, COUNT(*)::int AS friend_count
			FROM friendships f
			GROUP BY f.user_id
		)
		SELECT
			tu.id,
			tu.username,
			tu.created_at,
			COALESCE(SUM(rs.reps), 0)::bigint AS total_reps,
			COALESCE(s.streak_days, 0) AS streak_days,
			(founders.user_id IS NOT NULL) AS is_founder,
			COALESCE(fc.friend_count, 0) AS friend_count
		FROM target_user tu
		LEFT JOIN rep_sessions rs
		  ON rs.user_id = tu.id
		LEFT JOIN streaks s
		  ON s.user_id = tu.id
		LEFT JOIN founders
		  ON founders.user_id = tu.id
		LEFT JOIN friend_counts fc
		  ON fc.user_id = tu.id
		GROUP BY tu.id, tu.username, tu.created_at, s.streak_days, founders.user_id, fc.friend_count;
	`

	var (
		profileID   int64
		profile     profileResponse
		profileTime time.Time
	)
	err := dbPool.QueryRow(ctx, q, username).Scan(
		&profileID,
		&profile.Username,
		&profileTime,
		&profile.TotalReps,
		&profile.StreakDays,
		&profile.IsFounder,
		&profile.FriendsCount,
	)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}

	profile.CreatedAt = profileTime.UTC().Format(time.RFC3339)
	profile.IsSelf = profileID == viewerID

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

func handleDeleteOwnProfile(w http.ResponseWriter, r *http.Request) {
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const q = `
		DELETE FROM users
		WHERE id = $1;
	`

	tag, err := dbPool.Exec(ctx, q, userID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}

	_ = sessionMgr.Destroy(r.Context())

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
