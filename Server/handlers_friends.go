package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type friendRequest struct {
	Username string `json:"username"`
}

type friendRow struct {
	Username string `json:"username"`
}

type friendPendingRow struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	CreatedAt string `json:"createdAt"`
}

// RegisterFriendRoutes attaches friend-management endpoints under /api.
func RegisterFriendRoutes(r chi.Router) {
	r.Get("/friends", handleFriendsList)
	r.Post("/friends", handleCreateFriendRequest)
	r.Delete("/friends/{username}", handleRemoveFriend)
	r.Post("/friends/requests/{requestID}/accept", handleAcceptFriendRequest)
	r.Post("/friends/requests/{requestID}/deny", handleDenyFriendRequest)
	r.Delete("/friends/requests/{requestID}", handleCancelFriendRequest)
}

func handleFriendsList(w http.ResponseWriter, r *http.Request) {
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	friends, err := loadFriendRows(ctx, userID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	incoming, err := loadIncomingFriendRequests(ctx, userID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	outgoing, err := loadOutgoingFriendRequests(ctx, userID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"friends":          friends,
		"incomingRequests": incoming,
		"outgoingRequests": outgoing,
	})
}

func loadFriendRows(ctx context.Context, userID int64) ([]friendRow, error) {
	const q = `
		SELECT u.username
		FROM friendships f
		INNER JOIN users u ON u.id = f.friend_user_id
		WHERE f.user_id = $1
		ORDER BY u.username ASC;
	`

	rows, err := dbPool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	friends := make([]friendRow, 0)
	for rows.Next() {
		var fr friendRow
		if err := rows.Scan(&fr.Username); err != nil {
			return nil, err
		}
		friends = append(friends, fr)
	}

	return friends, rows.Err()
}

func loadIncomingFriendRequests(ctx context.Context, userID int64) ([]friendPendingRow, error) {
	const q = `
		SELECT fr.id, u.username, fr.created_at
		FROM friend_requests fr
		INNER JOIN users u ON u.id = fr.sender_user_id
		WHERE fr.receiver_user_id = $1
		  AND fr.status = 'pending'
		ORDER BY fr.created_at ASC, u.username ASC;
	`

	rows, err := dbPool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]friendPendingRow, 0)
	for rows.Next() {
		var (
			row       friendPendingRow
			createdAt time.Time
		)
		if err := rows.Scan(&row.ID, &row.Username, &createdAt); err != nil {
			return nil, err
		}
		row.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		requests = append(requests, row)
	}

	return requests, rows.Err()
}

func loadOutgoingFriendRequests(ctx context.Context, userID int64) ([]friendPendingRow, error) {
	const q = `
		SELECT fr.id, u.username, fr.created_at
		FROM friend_requests fr
		INNER JOIN users u ON u.id = fr.receiver_user_id
		WHERE fr.sender_user_id = $1
		  AND fr.status = 'pending'
		ORDER BY fr.created_at ASC, u.username ASC;
	`

	rows, err := dbPool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]friendPendingRow, 0)
	for rows.Next() {
		var (
			row       friendPendingRow
			createdAt time.Time
		)
		if err := rows.Scan(&row.ID, &row.Username, &createdAt); err != nil {
			return nil, err
		}
		row.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		requests = append(requests, row)
	}

	return requests, rows.Err()
}

func handleCreateFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	var req friendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(req.Username)
	if len(username) < 3 || len(username) > 32 || strings.ContainsAny(username, " \t\r\n") {
		http.Error(w, "username must be 3-32 characters with no spaces", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const targetUserQ = `
		SELECT id
		FROM users
		WHERE username = $1;
	`

	var targetUserID int64
	if err := dbPool.QueryRow(ctx, targetUserQ, username).Scan(&targetUserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "user does not exist", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if targetUserID == userID {
		http.Error(w, "cannot send a friend request to yourself", http.StatusBadRequest)
		return
	}

	const alreadyFriendsQ = `
		SELECT 1
		FROM friendships
		WHERE (user_id = $1 AND friend_user_id = $2)
		   OR (user_id = $2 AND friend_user_id = $1)
		LIMIT 1;
	`

	var exists int
	err := dbPool.QueryRow(ctx, alreadyFriendsQ, userID, targetUserID).Scan(&exists)
	if err == nil {
		http.Error(w, "already friends", http.StatusConflict)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	const pendingQ = `
		SELECT sender_user_id
		FROM friend_requests
		WHERE status = 'pending'
		  AND (
			(sender_user_id = $1 AND receiver_user_id = $2)
			OR
			(sender_user_id = $2 AND receiver_user_id = $1)
		  )
		LIMIT 1;
	`

	var pendingSenderID int64
	err = dbPool.QueryRow(ctx, pendingQ, userID, targetUserID).Scan(&pendingSenderID)
	if err == nil {
		if pendingSenderID == userID {
			http.Error(w, "friend request already sent", http.StatusConflict)
			return
		}
		http.Error(w, "this user already sent you a friend request", http.StatusConflict)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	const insertQ = `
		INSERT INTO friend_requests (sender_user_id, receiver_user_id, status)
		VALUES ($1, $2, 'pending');
	`

	_, err = dbPool.Exec(ctx, insertQ, userID, targetUserID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "friend request already pending", http.StatusConflict)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":          true,
		"requestSent": true,
	})
}

func handleAcceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	requestID, ok := parseRequestID(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := dbPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	const lockQ = `
		SELECT sender_user_id, receiver_user_id, status
		FROM friend_requests
		WHERE id = $1
		FOR UPDATE;
	`

	var (
		senderID   int64
		receiverID int64
		status     string
	)
	if err := tx.QueryRow(ctx, lockQ, requestID).Scan(&senderID, &receiverID, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "friend request not found", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if receiverID != userID {
		http.Error(w, "not authorized to accept this request", http.StatusForbidden)
		return
	}
	if status != "pending" {
		http.Error(w, "friend request already handled", http.StatusConflict)
		return
	}

	const insertFriendshipsQ = `
		INSERT INTO friendships (user_id, friend_user_id)
		VALUES ($1, $2), ($2, $1)
		ON CONFLICT DO NOTHING;
	`
	if _, err := tx.Exec(ctx, insertFriendshipsQ, senderID, receiverID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	const markAcceptedQ = `
		UPDATE friend_requests
		SET status = 'accepted', responded_at = NOW()
		WHERE id = $1;
	`
	if _, err := tx.Exec(ctx, markAcceptedQ, requestID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func handleDenyFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	requestID, ok := parseRequestID(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const q = `
		UPDATE friend_requests
		SET status = 'denied', responded_at = NOW()
		WHERE id = $1
		  AND receiver_user_id = $2
		  AND status = 'pending';
	`

	tag, err := dbPool.Exec(ctx, q, requestID, userID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "friend request not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func handleCancelFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	requestID, ok := parseRequestID(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const q = `
		UPDATE friend_requests
		SET status = 'cancelled', responded_at = NOW()
		WHERE id = $1
		  AND sender_user_id = $2
		  AND status = 'pending';
	`

	tag, err := dbPool.Exec(ctx, q, requestID, userID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "friend request not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func handleRemoveFriend(w http.ResponseWriter, r *http.Request) {
	userID := int64(sessionMgr.GetInt(r.Context(), "userID"))
	if userID == 0 {
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
		WITH target AS (
			SELECT id
			FROM users
			WHERE username = $2
		)
		DELETE FROM friendships
		WHERE (user_id = $1 AND friend_user_id IN (SELECT id FROM target))
		   OR (friend_user_id = $1 AND user_id IN (SELECT id FROM target));
	`

	tag, err := dbPool.Exec(ctx, q, userID, username)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "friend not found in your list", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func parseRequestID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	idText := strings.TrimSpace(chi.URLParam(r, "requestID"))
	requestID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || requestID <= 0 {
		http.Error(w, "invalid request id", http.StatusBadRequest)
		return 0, false
	}

	return requestID, true
}
