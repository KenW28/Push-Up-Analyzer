package main

import (
	"context"
	"testing"
)

func TestMemoryAuthStore_CreateAndGet(t *testing.T) {
	store := NewMemoryAuthStore()

	u, err := store.Create(context.Background(), "ken", "hash")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.GetByUsername(context.Background(), "ken")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.ID != u.ID || got.Username != u.Username || got.PasswordHash != u.PasswordHash {
		t.Fatalf("mismatch: %+v vs %+v", got, u)
	}
}

func TestMemoryAuthStore_DuplicateUsername(t *testing.T) {
	store := NewMemoryAuthStore()

	if _, err := store.Create(context.Background(), "ken", "hash"); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := store.Create(context.Background(), "ken", "hash2"); err != ErrUsernameTaken {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
}
