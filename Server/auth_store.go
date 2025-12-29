package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrUsernameTaken = errors.New("username already taken")
)

type AuthUser struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type AuthStore interface {
	Create(ctx context.Context, username, passwordHash string) (AuthUser, error)
	GetByUsername(ctx context.Context, username string) (AuthUser, error)
}

// ---- In-memory implementation (dev fallback) ----

type MemoryAuthStore struct {
	mu    sync.Mutex
	next  int64
	users map[string]AuthUser // keyed by username
}

func NewMemoryAuthStore() *MemoryAuthStore {
	return &MemoryAuthStore{
		next:  1,
		users: make(map[string]AuthUser),
	}
}

func (s *MemoryAuthStore) Create(ctx context.Context, username, passwordHash string) (AuthUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[username]; exists {
		return AuthUser{}, ErrUsernameTaken
	}

	u := AuthUser{
		ID:           s.next,
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}
	s.next++
	s.users[username] = u
	return u, nil
}

func (s *MemoryAuthStore) GetByUsername(ctx context.Context, username string) (AuthUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[username]
	if !ok {
		return AuthUser{}, ErrUserNotFound
	}
	return u, nil
}
