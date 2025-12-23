package main

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// LoginLimiter slows down brute-force password guessing.
// It tracks login attempts per IP address in a time window.
//
// Note: This is memory-based. If the server restarts, counters reset.
// For bigger production deployments, you’d use a shared store (Redis) or a CDN/WAF.
type LoginLimiter struct {
	mu        sync.Mutex
	hits      map[string][]time.Time
	limit     int
	windowDur time.Duration
}

func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		hits:      make(map[string][]time.Time),
		limit:     10,              // 10 attempts
		windowDur: 2 * time.Minute, // per 2 minutes
	}
}

func (l *LoginLimiter) Allow(r *http.Request) bool {
	ip := clientIP(r)

	now := time.Now()
	cutoff := now.Add(-l.windowDur)

	l.mu.Lock()
	defer l.mu.Unlock()

	list := l.hits[ip]

	// Drop timestamps older than the window.
	j := 0
	for _, t := range list {
		if t.After(cutoff) {
			list[j] = t
			j++
		}
	}
	list = list[:j]

	// If too many attempts remain in the window, deny.
	if len(list) >= l.limit {
		l.hits[ip] = list
		return false
	}

	list = append(list, now)
	l.hits[ip] = list
	return true
}

func clientIP(r *http.Request) string {
	// For local dev, RemoteAddr is fine.
	// Behind reverse proxies you’d handle X-Forwarded-For carefully.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
