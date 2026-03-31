package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

var store AuthStore
var dbPool *pgxpool.Pool

// sessionMgr handles secure session cookies.
var sessionMgr = scs.New()

// loginLimiter slows brute-force login attempts.
var loginLimiter = NewLoginLimiter()

// main is the entry point of your Go program.
// Think of it like: "set up the server, then start listening for web requests."
func main() {

	// Create the router. The router decides which handler function runs for each URL.
	r := chi.NewRouter()

	// This middleware reads/writes the session cookie on every request.
	r.Use(sessionMgr.LoadAndSave)

	// Middleware = code that runs for every request.
	// Logger prints request information so you can debug easily.
	r.Use(middleware.Logger)

	// Recoverer prevents a crash from killing the server if a handler panics.
	r.Use(middleware.Recoverer)

	// Security headers middleware: add CSP, X-Frame-Options, etc.
	r.Use(securityHeadersMiddleware)

	// Sessions timeout: read from environment, default to 4 hours.
	timeoutHours := 4
	if val := os.Getenv("SESSION_TIMEOUT_HOURS"); val != "" {
		if h, err := strconv.Atoi(val); err == nil && h > 0 {
			timeoutHours = h
		}
	}
	sessionMgr.Lifetime = time.Duration(timeoutHours) * time.Hour

	// Security properties:
	// - HttpOnly: JS cannot read the cookie (helps against XSS stealing sessions).
	// - SameSite Lax: blocks most cross-site request forgery (CSRF) attempts.
	// - Secure: MUST be true in production (requires HTTPS). For localhost dev, false is OK.
	isProduction := os.Getenv("ENV") == "production"
	sessionMgr.Cookie.HttpOnly = true
	sessionMgr.Cookie.SameSite = http.SameSiteLaxMode
	sessionMgr.Cookie.Secure = isProduction // Only send cookie over HTTPS in production

	// Timeout puts an upper bound on request handling time.
	// This prevents a request from hanging forever.
	r.Use(middleware.Timeout(10 * time.Second))

	dbPool = openDB()
	defer dbPool.Close()

	store = NewPostgresAuthStore(dbPool)

	// --- API ROUTES ---
	// We group all API endpoints under /api
	r.Route("/api", func(api chi.Router) {
		// Health endpoint: quick way to confirm server is running.
		api.Get("/health", handleHealth)

		// Register route groups defined in other files.
		RegisterAuthRoutes(api)
		RegisterFriendRoutes(api)
		RegisterDeviceTokenRoutes(api)
		RegisterProfileRoutes(api)
		RegisterLeaderboardRoutes(api)
		RegisterRepRoutes(api)
	})

	// --- STATIC FRONTEND FILES ---
	// This serves your HTML/CSS/JS from the ../public folder.
	// When the browser requests "/", it will load index.html from that folder.
	// --- STATIC FRONTEND FILES ---
	publicDir := filepath.Join("..", "Frontend")
	fileServer := http.FileServer(http.Dir(publicDir))

	// Protect ONLY "/" so unauth users get redirected before index.html loads (no "flash").
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		protected := requireLoginRedirect(sessionMgr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(publicDir, "index.html"))
		}))
		protected.ServeHTTP(w, r)
	})

	r.Get("/friends.html", func(w http.ResponseWriter, r *http.Request) {
		protected := requireLoginRedirect(sessionMgr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(publicDir, "friends.html"))
		}))
		protected.ServeHTTP(w, r)
	})

	r.Get("/profile.html", func(w http.ResponseWriter, r *http.Request) {
		protected := requireLoginRedirect(sessionMgr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(publicDir, "profile.html"))
		}))
		protected.ServeHTTP(w, r)
	})

	// Everything else (login.html, register.html, css, js, etc.) stays publicly accessible.
	r.Handle("/*", fileServer)

	// Start the server on port 3000.
	addr := ":3000"
	log.Println("Server listening at http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

// securityHeadersMiddleware adds security headers to all responses.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME-type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking attacks
		w.Header().Set("X-Frame-Options", "DENY")

		// Basic Content Security Policy (allows same-origin resources only)
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' fonts.googleapis.com; font-src fonts.gstatic.com")

		// HSTS: force HTTPS for 1 year (only in production)
		if os.Getenv("ENV") == "production" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// handleHealth is a very small endpoint for confirming the server is alive.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// requireLoginRedirect redirects to /login.html if the user is not logged in.
func requireLoginRedirect(session *scs.SessionManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := session.GetInt(r.Context(), "userID")
		if userID == 0 {
			http.Redirect(w, r, "/login.html", http.StatusFound) // 302
			return
		}
		next.ServeHTTP(w, r)
	})
}
