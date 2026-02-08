package main

import (
	"testing"
	"time"
)

func TestLeaderboardWindowStart(t *testing.T) {
	now := time.Date(2026, 1, 9, 12, 0, 0, 0, time.UTC)

	got := leaderboardWindowStart(now, "minute")
	if got != now.Add(-1*time.Minute) {
		t.Fatalf("minute window mismatch: %v", got)
	}

	got = leaderboardWindowStart(now, "30s")
	if got != now.Add(-30*time.Second) {
		t.Fatalf("30s window mismatch: %v", got)
	}

	got = leaderboardWindowStart(now, "month")
	if got != now.AddDate(0, -1, 0) {
		t.Fatalf("month window mismatch: %v", got)
	}
}
