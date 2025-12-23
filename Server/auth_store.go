package main

import (
	"errors"
	"sync"
)

// User is the minimal account record.
// We store a bcrypt hash, NEVER the plaintext password.
type User struct {
	ID           int
	Username     string
	PasswordHash []byte
}

// UserStore is a simple in-memory store.
// WARNING: data disappears when the server restarts.
// Later we can swap this to a real DB with the same method signatures.
type UserStore struct {
	mu     sync.RWMutex
	nextID int
	byName map[string]User
}

var (
	ErrUserExists   = errors.New("username already taken")
	ErrUserNotFound = errors.New("user not found")
)

func NewUserStore() *UserStore {
	return &UserStore{
		nextID: 1,
		byName: make(map[string]User),
	}
}

func (s *UserStore) Create(username string, passwordHash []byte) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byName[username]; exists {
		return User{}, ErrUserExists
	}

	u := User{
		ID:           s.nextID,
		Username:     username,
		PasswordHash: passwordHash,
	}
	s.nextID++
	s.byName[username] = u
	return u, nil
}

func (s *UserStore) GetByUsername(username string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.byName[username]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}
