/**
 * Ask the backend who is currently logged in (based on the session cookie).
 * If not logged in, redirect to /login.html.
 */
function refreshAuthUI() {
  const label = document.getElementById("auth-label");
  const logoutBtn = document.getElementById("logout-btn");

  // If the auth UI isn't on this page, don't crash.
  if (!label || !logoutBtn) return Promise.resolve(false);

  return fetch("/api/auth/me", { headers: { "Accept": "application/json" } })
    .then((r) => {
      if (!r.ok) throw new Error("Failed to check auth state");
      return r.json();
    })
    .then((payload) => {
      if (!payload.loggedIn) {
        window.location.href = "/login.html";
        return false;
      }

      label.textContent = `Logged in as: ${payload.username}`;
      logoutBtn.style.display = "inline-block";
      return true;
    })
    .catch((err) => {
      console.error("Auth check failed:", err);
      if (label) label.textContent = "Auth check failed";
      return false;
    });
}


/**
 * Logs out by asking the backend to destroy the session.
 * After logout, redirect to login page.
 */
function logout() {
  const logoutBtn = document.getElementById("logout-btn");
  if (logoutBtn) logoutBtn.disabled = true;

  fetch("/api/auth/logout", { method: "POST" })
    .then(() => {
      window.location.href = "/login.html";
    })
    .catch((err) => {
      console.error(err);
      if (logoutBtn) logoutBtn.disabled = false;
      alert("Logout failed. Try again.");
    });
}



// This function fetches leaderboard data from the backend API with the given filters.
/**
 * Fetch leaderboard rows from the backend API.
 *
 * The backend returns a JSON payload like:
 * {
 *   "scope": "global",
 *   "window": "month",
 *   "rows": [
 *     { "username": "kendrick", "totalReps": 840 },
 *     ...
 *   ]
 * }
 *
 * This function returns ONLY the rows array because the UI renderer
 * only cares about rows.
 */
function getLeaderboardData(scope, windowKey) {
  // Build the query string safely
  const url = `/api/leaderboard?scope=${encodeURIComponent(scope)}&window=${encodeURIComponent(windowKey)}`;

  return fetch(url, {
    method: "GET",
    headers: {
      // Not strictly required, but makes intent clear
      "Accept": "application/json",
    },
  })
    .then((response) => {
      if (response.status === 401) {
      window.location.href = "/login.html";
      throw new Error("Unauthorized");
  }

      // this is where you'll detect it and show a login message.
      if (!response.ok) {
        throw new Error(`Leaderboard API failed: ${response.status} ${response.statusText}`);
      }
      return response.json();
    })
    .then((payload) => {
      // Defensive: ensure we always return an array
      if (!payload || !Array.isArray(payload.rows)) return [];
      return payload.rows;
    });
}


// Current filter state
let currentScope = "global"; // "global" or "friends"
let currentWindow = "month"; // "month", "minute", "30s"





function renderLeaderboard(rows) {
  const tbody = document.getElementById("leaderboard-body");
  const updatedLabel = document.getElementById("last-updated");

  if (!tbody) return;

  // Clear existing rows
  tbody.innerHTML = "";

  rows.forEach((row, index) => {
    const tr = document.createElement("tr");

    const rankTd = document.createElement("td");
    rankTd.textContent = index + 1;
    rankTd.classList.add("rank");

    if (index === 0) rankTd.classList.add("rank-1");
    if (index === 1) rankTd.classList.add("rank-2");
    if (index === 2) rankTd.classList.add("rank-3");

    const userTd = document.createElement("td");
    userTd.textContent = row.username;

    const repsTd = document.createElement("td");
    repsTd.textContent = row.totalReps;

    tr.appendChild(rankTd);
    tr.appendChild(userTd);
    tr.appendChild(repsTd);

    tbody.appendChild(tr);
  });

  if (updatedLabel) {
    const now = new Date();
    updatedLabel.textContent = `Last updated: ${now.toLocaleString()}`;
  }
  

}
function refreshLeaderboard() {
  getLeaderboardData(currentScope, currentWindow)
    .then((data) => {
      // sort descending by score
      const sorted = [...data].sort((a, b) => b.totalReps - a.totalReps);
      renderLeaderboard(sorted);
    })
    .catch((err) => {
      console.error("Failed to load leaderboard:", err);
      const tbody = document.getElementById("leaderboard-body");
      if (tbody) {
        tbody.innerHTML = "";
        const tr = document.createElement("tr");
        const td = document.createElement("td");
        td.colSpan = 3;
        td.textContent = "Error loading leaderboard.";
        tr.appendChild(td);
        tbody.appendChild(tr);
      }
    });
}

// Initialization (runs once after the HTML loads)
document.addEventListener("DOMContentLoaded", () => {
  const scopeSelect = document.getElementById("scope-select");
  const windowSelect = document.getElementById("window-select");
  const logoutBtn = document.getElementById("logout-btn");

  // Wire dropdown: Scope
  if (scopeSelect) {
    scopeSelect.value = currentScope;
    scopeSelect.addEventListener("change", () => {
      currentScope = scopeSelect.value;
      refreshLeaderboard();
    });
  }

  // Wire dropdown: Time window
  if (windowSelect) {
    windowSelect.value = currentWindow;
    windowSelect.addEventListener("change", () => {
      currentWindow = windowSelect.value;
      refreshLeaderboard();
    });
  }

  // Wire logout button click
  if (logoutBtn) {
    logoutBtn.addEventListener("click", logout);
  }

  // Confirm login first. If not logged in, this will redirect to /login.html.
  // Only load leaderboard after auth is confirmed.
  refreshAuthUI().then((ok) => {
    if (ok) refreshLeaderboard();
  });
});


