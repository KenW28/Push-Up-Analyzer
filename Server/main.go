package main

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var store AuthStore

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

	// Sessions live for 24 hours (adjust later).
	sessionMgr.Lifetime = 24 * time.Hour

	// Security properties:
	// - HttpOnly: JS cannot read the cookie (helps against XSS stealing sessions).
	// - SameSite Lax: blocks most cross-site request forgery (CSRF) attempts.
	// - Secure: MUST be true in production (requires HTTPS). For localhost dev, false is OK.
	sessionMgr.Cookie.HttpOnly = true
	sessionMgr.Cookie.SameSite = http.SameSiteLaxMode
	sessionMgr.Cookie.Secure = false // TODO: set true in production (HTTPS)

	// Timeout puts an upper bound on request handling time.
	// This prevents a request from hanging forever.
	r.Use(middleware.Timeout(10 * time.Second))

	db := openDB()
	defer db.Close()

	store = NewPostgresAuthStore(db)

	// --- API ROUTES ---
	// We group all API endpoints under /api
	r.Route("/api", func(api chi.Router) {
		// Health endpoint: quick way to confirm server is running.
		api.Get("/health", handleHealth)

		// Register route groups defined in other files.
		RegisterAuthRoutes(api)
		RegisterLeaderboardRoutes(api)
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

	// Everything else (login.html, register.html, css, js, etc.) stays publicly accessible.
	r.Handle("/*", fileServer)

	// Start the server on port 3000.
	addr := ":3000"
	log.Println("Server listening at http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, r))
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
