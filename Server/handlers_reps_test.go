package main

import (
	"strings"
	"testing"
)

func TestHashDeviceToken(t *testing.T) {
	hash := hashDeviceToken("test-token")

	if len(hash) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(hash))
	}
	if strings.ContainsAny(hash, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Fatalf("hash should be lowercase hex: %s", hash)
	}
}
