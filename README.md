# Push-Up-Analyzer
Early build of Pressle. A social push-ups application and hardware to track reps while showing a live leaderboard. This is the webpage and database development repo.

## Project layout
Frontend/
- index.html main page layout
- styles.css colors, fonts, spacing
- app.js page behavior and API calls

Server/
- main.go server entry point
- handlers_auth.go login and sessions
- handlers_leaderboard.go leaderboard API
- handlers_reps.go reps API
- db.go database connection
- migrations/ database tables

docs/
- simple landing page for pressle.app
