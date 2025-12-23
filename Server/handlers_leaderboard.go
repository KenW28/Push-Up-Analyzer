package main

import (
	"encoding/json"
	"net/http"
	"sort"

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

	// Demo base data. Later this comes from persistence.
	base := []struct {
		Username string
		BaseWeek int
		IsFriend bool
	}{
		{"kendrick", 210, true},
		{"alex", 180, true},
		{"jordan", 145, false},
		{"taylor", 120, true},
		{"sam", 90, false},
	}

	// Apply "friends only" filter if requested.
	filtered := make([]struct {
		Username string
		BaseWeek int
		IsFriend bool
	}, 0, len(base))

	for _, p := range base {
		if scope == "friends" && !p.IsFriend {
			continue
		}
		filtered = append(filtered, p)
	}

	// Map base data into leaderboard rows (username + computed reps)
	rows := make([]LeaderboardRow, 0, len(filtered))
	for _, p := range filtered {
		rows = append(rows, LeaderboardRow{
			Username:  p.Username,
			TotalReps: computeRepsForWindow(p.BaseWeek, windowKey),
		})
	}

	// Sort descending by TotalReps
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].TotalReps > rows[j].TotalReps
	})

	// Respond as JSON
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"scope":  scope,
		"window": windowKey,
		"rows":   rows,
	})
}

// computeRepsForWindow matches your frontend demo logic.
// Later, you may move this logic to the DB query or keep it here.
func computeRepsForWindow(baseWeek int, windowKey string) int {
	switch windowKey {
	case "month":
		return baseWeek * 4
	case "minute":
		v := baseWeek / 200
		if v < 1 {
			return 1
		}
		return v
	case "30s":
		v := baseWeek / 400
		if v < 1 {
			return 1
		}
		return v
	default:
		return baseWeek
	}
}
