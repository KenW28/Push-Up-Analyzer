package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// LeaderboardRow is the exact row shape the frontend expects.
type LeaderboardRow struct {
	Username   string `json:"username"`
	TotalReps  int    `json:"totalReps"`
	StreakDays int    `json:"streakDays"`
	IsFounder  bool   `json:"isFounder"`
}

// RegisterLeaderboardRoutes attaches leaderboard endpoints under /api.
func RegisterLeaderboardRoutes(r chi.Router) {
	r.Get("/leaderboard", handleLeaderboard)
}

// handleLeaderboard returns leaderboard data from Postgres.
func handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	// Require login: if no session userID, deny.
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	// Read query parameters from the URL: ?scope=global&window=month
	scope := r.URL.Query().Get("scope")
	if scope != "friends" && scope != "global" {
		scope = "global"
	}

	windowKey := r.URL.Query().Get("window")
	if windowKey == "" {
		windowKey = "month"
	}

	rows, err := loadLeaderboardRows(r.Context(), windowKey, userID, scope)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// Respond as JSON
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"scope":  scope,
		"window": windowKey,
		"rows":   rows,
	})
}

func loadLeaderboardRows(ctx context.Context, windowKey string, userID int64, scope string) ([]LeaderboardRow, error) {
	windowStart := leaderboardWindowStart(time.Now().UTC(), windowKey)

	query := `
		WITH daily_activity AS (
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
		)
		SELECT
			u.username,
			COALESCE(SUM(rs.reps), 0) AS total_reps,
			COALESCE(s.streak_days, 0) AS streak_days,
			(f.user_id IS NOT NULL) AS is_founder
		FROM users u
		LEFT JOIN rep_sessions rs
		  ON rs.user_id = u.id
		 AND rs.created_at >= $1
		LEFT JOIN streaks s
		  ON s.user_id = u.id
		LEFT JOIN founders f
		  ON f.user_id = u.id
		GROUP BY u.id, u.username, s.streak_days, f.user_id
		ORDER BY total_reps DESC, u.username ASC;
	`
	args := []any{windowStart}

	if scope == "friends" {
		query = `
			WITH daily_activity AS (
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
			)
			SELECT
				u.username,
				COALESCE(SUM(rs.reps), 0) AS total_reps,
				COALESCE(s.streak_days, 0) AS streak_days,
				(f.user_id IS NOT NULL) AS is_founder
			FROM users u
			LEFT JOIN rep_sessions rs
			  ON rs.user_id = u.id
			 AND rs.created_at >= $1
			LEFT JOIN streaks s
			  ON s.user_id = u.id
			LEFT JOIN founders f
			  ON f.user_id = u.id
			WHERE u.id = $2
			   OR EXISTS (
					SELECT 1
					FROM friendships f
					WHERE f.user_id = $2
					  AND f.friend_user_id = u.id
			   )
			GROUP BY u.id, u.username, s.streak_days, f.user_id
			ORDER BY total_reps DESC, u.username ASC;
		`
		args = append(args, userID)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := dbPool.Query(queryCtx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]LeaderboardRow, 0)
	for rows.Next() {
		var row LeaderboardRow
		if err := rows.Scan(&row.Username, &row.TotalReps, &row.StreakDays, &row.IsFounder); err != nil {
			return nil, err
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

func leaderboardWindowStart(now time.Time, windowKey string) time.Time {
	switch windowKey {
	case "minute":
		return now.Add(-1 * time.Minute)
	case "30s":
		return now.Add(-30 * time.Second)
	case "month":
		return now.AddDate(0, -1, 0)
	default:
		return now.AddDate(0, -1, 0)
	}
}
