// In a real version, this will come from your backend via fetch().
// For now, we simulate an API response to keep it simple and lightweight.
// Base data that does not depend on filters.
// In reality this would come from your backend.
const baseLeaderboard = [
  { username: "kendrick", baseWeek: 210, isFriend: true },
  { username: "alex", baseWeek: 180, isFriend: true },
  { username: "jordan", baseWeek: 145, isFriend: false },
  { username: "taylor", baseWeek: 120, isFriend: true },
  { username: "sam", baseWeek: 90, isFriend: false },
];

// Current filter state
let currentScope = "global"; // "global" or "friends"
let currentWindow = "month"; // "month", "minute", "30s"

// This function turns "baseWeek" plus the selected time window into a number of reps.
// These numbers are just for demo. Later this logic will live in your backend.
function computeRepsForWindow(baseWeek, windowKey) {
  switch (windowKey) {
    case "month":
      // rough: about 4 weeks in a month
      return baseWeek * 4;
    case "minute":
      // how many you might do in a very intense minute
      return Math.max(1, Math.round(baseWeek / 200));
    case "30s":
      // half of the one minute count, roughly
      return Math.max(1, Math.round(baseWeek / 400));
    default:
      return baseWeek;
  }
}

// This function applies the filters and returns data for the table.
function getLeaderboardData(scope, windowKey) {
  let filtered = baseLeaderboard;

  // Scope filter: include only "friends" if requested
  if (scope === "friends") {
    filtered = filtered.filter((p) => p.isFriend);
  }

  // Map base data into something with a score for the selected time window
  const mapped = filtered.map((p) => ({
    username: p.username,
    totalReps: computeRepsForWindow(p.baseWeek, windowKey),
  }));

  // In a real app this would be an async fetch.
  // We wrap it in a Promise so the calling code looks the same.
  return Promise.resolve(mapped);
}


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

// Initialization
document.addEventListener("DOMContentLoaded", () => {
  const scopeSelect = document.getElementById("scope-select");
  const windowSelect = document.getElementById("window-select");

  if (scopeSelect) {
    scopeSelect.value = currentScope;
    scopeSelect.addEventListener("change", () => {
      currentScope = scopeSelect.value;
      refreshLeaderboard();
    });
  }

  if (windowSelect) {
    windowSelect.value = currentWindow;
    windowSelect.addEventListener("change", () => {
      currentWindow = windowSelect.value;
      refreshLeaderboard();
    });
  }

  // Initial render
  refreshLeaderboard();
});

