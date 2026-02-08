package main

import (
	"net/http"
	"testing"
	"time"
)

func TestLoginLimiter_AllowsThenBlocks(t *testing.T) {
	lim := NewLoginLimiter()
	lim.limit = 2

	r := &http.Request{RemoteAddr: "127.0.0.1:12345"}

	if !lim.Allow(r) {
		t.Fatal("first attempt should be allowed")
	}
	if !lim.Allow(r) {
		t.Fatal("second attempt should be allowed")
	}
	if lim.Allow(r) {
		t.Fatal("third attempt should be blocked")
	}
}

func TestLoginLimiter_WindowExpires(t *testing.T) {
	lim := NewLoginLimiter()
	lim.limit = 1
	lim.windowDur = 10 * time.Millisecond

	r := &http.Request{RemoteAddr: "127.0.0.1:12345"}

	if !lim.Allow(r) {
		t.Fatal("first attempt should be allowed")
	}
	if lim.Allow(r) {
		t.Fatal("second attempt should be blocked")
	}

	time.Sleep(15 * time.Millisecond)

	if !lim.Allow(r) {
		t.Fatal("attempt after window should be allowed")
	}
}
