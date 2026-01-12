package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
)

// LeaderboardRow is the exact row shape the frontend expects.
type LeaderboardRow struct {
	Username  string `json:"username"`
	TotalReps int    `json:"totalReps"`
}

// RegisterLeaderboardRoutes attaches leaderboard endpoints under /api.
func RegisterLeaderboardRoutes(r chi.Router) {
	r.Get("/leaderboard", handleLeaderboard)
}

// handleLeaderboard returns leaderboard data.
// For now, it's hardcoded demo data. Later, it will come from the database.
func handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	// Require login: if no session userID, deny.
	if sessionMgr.GetInt(r.Context(), "userID") == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	// Read query parameters from the URL: ?scope=global&window=month
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "global"
	}

	windowKey := r.URL.Query().Get("window")
	if windowKey == "" {
		windowKey = "month"
	}

	rows, err := loadLeaderboardRows(r.Context(), windowKey)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// Temporary: no friends model yet, so "friends" uses the same data.
	if scope == "friends" {
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].TotalReps > rows[j].TotalReps
		})
	}

	// Respond as JSON
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"scope":  scope,
		"window": windowKey,
		"rows":   rows,
	})
}

func loadLeaderboardRows(ctx context.Context, windowKey string) ([]LeaderboardRow, error) {
	windowStart := leaderboardWindowStart(time.Now().UTC(), windowKey)

	const q = `
		SELECT u.username, COALESCE(SUM(rs.reps), 0) AS total_reps
		FROM users u
		LEFT JOIN rep_sessions rs
		  ON rs.user_id = u.id
		 AND rs.created_at >= $1
		GROUP BY u.username
		ORDER BY total_reps DESC, u.username ASC;
	`

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := dbPool.Query(queryCtx, q, windowStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]LeaderboardRow, 0)
	for rows.Next() {
		var row LeaderboardRow
		if err := rows.Scan(&row.Username, &row.TotalReps); err != nil {
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
