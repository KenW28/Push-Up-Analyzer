package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"
)

// authRequest is the JSON shape sent from login/register pages.
type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func RegisterAuthRoutes(r chi.Router) {
	r.Route("/auth", func(auth chi.Router) {
		auth.Post("/register", handleRegister)
		auth.Post("/login", handleLogin)
		auth.Post("/logout", handleLogout)
		auth.Get("/me", handleMe)
	})
}

// handleRegister creates a user account.
// SAFETY: we hash passwords with bcrypt and never store plaintext.
func handleRegister(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(req.Username)
	password := req.Password

	// Minimal rules to avoid junk accounts.
	if len(username) < 3 {
		http.Error(w, "username must be at least 3 characters", http.StatusBadRequest)
		return
	}
	if len(password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Hash the password (slow by design to resist cracking).
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	_, err = store.Create(username, hash)
	if err == ErrUserExists {
		http.Error(w, "username already taken", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// NOTE: we do NOT auto-login on register (simpler mental model).
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte("ok"))
}

// handleLogin verifies credentials and creates a session cookie.
func handleLogin(w http.ResponseWriter, r *http.Request) {
	// Rate limit to slow brute force attacks.
	if !loginLimiter.Allow(r) {
		http.Error(w, "too many attempts, try again soon", http.StatusTooManyRequests)
		return
	}

	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(req.Username)
	password := req.Password

	// SAFETY: Avoid leaking whether a username exists.
	// Always return "invalid credentials" on failure.
	u, err := store.GetByUsername(username)
	if err != nil {
		// tiny delay makes username probing harder to measure
		time.Sleep(150 * time.Millisecond)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Success: set session values.
	sessionMgr.Put(r.Context(), "userID", u.ID)
	sessionMgr.Put(r.Context(), "username", u.Username)

	_, _ = w.Write([]byte("ok"))
}

// handleLogout clears the session.
func handleLogout(w http.ResponseWriter, r *http.Request) {
	_ = sessionMgr.Destroy(r.Context())
	_, _ = w.Write([]byte("ok"))
}

// handleMe is a convenience endpoint to check login state.
func handleMe(w http.ResponseWriter, r *http.Request) {
	username := sessionMgr.GetString(r.Context(), "username")
	userID := sessionMgr.GetInt(r.Context(), "userID")

	w.Header().Set("Content-Type", "application/json")

	if userID == 0 || username == "" {
		_ = json.NewEncoder(w).Encode(map[string]any{"loggedIn": false})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"loggedIn": true,
		"username": username,
	})
}
