/**
 * Ask the backend who is currently logged in (based on the session cookie).
 * If not logged in, redirect to /login.html.
 */
function refreshAuthUI() {
  const label = document.getElementById("auth-label");
  const logoutBtn = document.getElementById("logout-btn");

  // If the auth UI isn't on this page, don't crash.
  if (!label || !logoutBtn) return Promise.resolve(false);

  return fetch("/api/auth/me", { headers: { Accept: "application/json" } })
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

/**
 * Fetch leaderboard rows from the backend API.
 */
function getLeaderboardData(scope, windowKey) {
  const url = `/api/leaderboard?scope=${encodeURIComponent(scope)}&window=${encodeURIComponent(windowKey)}`;

  return fetch(url, {
    method: "GET",
    headers: {
      Accept: "application/json",
    },
  })
    .then((response) => {
      if (response.status === 401) {
        window.location.href = "/login.html";
        throw new Error("Unauthorized");
      }

      if (!response.ok) {
        throw new Error(`Leaderboard API failed: ${response.status} ${response.statusText}`);
      }
      return response.json();
    })
    .then((payload) => {
      if (!payload || !Array.isArray(payload.rows)) return [];
      return payload.rows;
    });
}

let currentScope = "global";
let currentWindow = "month";
let closeProfileMenu = null;

function openProfileWindow(username) {
  const width = 520;
  const height = 680;
  const left = Math.max(0, window.screenX + Math.round((window.outerWidth - width) / 2));
  const top = Math.max(0, window.screenY + Math.round((window.outerHeight - height) / 2));
  const features = [
    `width=${width}`,
    `height=${height}`,
    `left=${left}`,
    `top=${top}`,
    "resizable=yes",
    "scrollbars=yes",
  ].join(",");

  const url = `/profile.html?username=${encodeURIComponent(username)}`;
  const popup = window.open(url, "pressle-profile", features);
  if (!popup) {
    window.location.href = url;
    return;
  }

  popup.focus();
}

function openProfileMenu(anchorEl, username) {
  if (closeProfileMenu) closeProfileMenu();

  const menu = document.createElement("div");
  menu.className = "profile-action-menu";

  const viewProfileBtn = document.createElement("button");
  viewProfileBtn.type = "button";
  viewProfileBtn.textContent = "View profile";
  viewProfileBtn.addEventListener("click", (event) => {
    event.preventDefault();
    event.stopPropagation();
    if (closeProfileMenu) closeProfileMenu();
    openProfileWindow(username);
  });

  menu.appendChild(viewProfileBtn);
  document.body.appendChild(menu);

  const rect = anchorEl.getBoundingClientRect();
  const menuWidth = menu.offsetWidth || 170;
  const left = Math.min(
    window.scrollX + rect.left,
    window.scrollX + window.innerWidth - menuWidth - 12,
  );
  menu.style.top = `${window.scrollY + rect.bottom + 6}px`;
  menu.style.left = `${Math.max(window.scrollX + 8, left)}px`;

  const closeHandler = () => {
    if (!menu.isConnected) return;
    menu.remove();
  };

  const onDocumentClick = (event) => {
    if (!menu.contains(event.target) && !anchorEl.contains(event.target)) {
      if (closeProfileMenu) closeProfileMenu();
    }
  };
  const onKeyDown = (event) => {
    if (event.key === "Escape" && closeProfileMenu) closeProfileMenu();
  };
  const onScroll = () => {
    if (closeProfileMenu) closeProfileMenu();
  };

  document.addEventListener("click", onDocumentClick);
  window.addEventListener("keydown", onKeyDown);
  window.addEventListener("scroll", onScroll, true);
  window.addEventListener("resize", onScroll);

  closeProfileMenu = () => {
    document.removeEventListener("click", onDocumentClick);
    window.removeEventListener("keydown", onKeyDown);
    window.removeEventListener("scroll", onScroll, true);
    window.removeEventListener("resize", onScroll);
    closeHandler();
    closeProfileMenu = null;
  };
}

function createFounderIcon() {
  const svgNS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(svgNS, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("aria-hidden", "true");
  svg.classList.add("founder-icon");

  const path = document.createElementNS(svgNS, "path");
  path.setAttribute("d", "M2 18 L6 12 L9 14 L12 9 L15 13 L18 10 L22 18 Z");
  path.setAttribute("fill", "none");
  path.setAttribute("stroke", "currentColor");
  path.setAttribute("stroke-width", "1.8");
  path.setAttribute("stroke-linecap", "round");
  path.setAttribute("stroke-linejoin", "round");

  svg.appendChild(path);
  return svg;
}

function renderUserCell(row) {
  const wrap = document.createElement("div");
  wrap.className = "leaderboard-user";

  const usernameBtn = document.createElement("button");
  usernameBtn.type = "button";
  usernameBtn.className = "leaderboard-username-btn";
  usernameBtn.textContent = row.username;
  usernameBtn.addEventListener("click", (event) => {
    event.preventDefault();
    event.stopPropagation();
    openProfileMenu(usernameBtn, row.username);
  });
  wrap.appendChild(usernameBtn);

  const streakDays = Number(row.streakDays || 0);
  if (streakDays >= 5) {
    const streakBadge = document.createElement("span");
    streakBadge.className = "user-badge streak-badge";
    streakBadge.textContent = `${streakDays}d streak`;
    wrap.appendChild(streakBadge);
  }

  if (row.isFounder) {
    const founderBadge = document.createElement("span");
    founderBadge.className = "user-badge founder-badge";
    founderBadge.title = "Founding Fathers badge (first 50 device token registrations)";

    founderBadge.appendChild(createFounderIcon());

    const label = document.createElement("span");
    label.textContent = "Founder";
    founderBadge.appendChild(label);

    wrap.appendChild(founderBadge);
  }

  return wrap;
}

function renderLeaderboard(rows) {
  const tbody = document.getElementById("leaderboard-body");
  const updatedLabel = document.getElementById("last-updated");

  if (!tbody) return;

  if (closeProfileMenu) closeProfileMenu();

  tbody.innerHTML = "";

  if (rows.length === 0) {
    const tr = document.createElement("tr");
    const td = document.createElement("td");
    td.colSpan = 3;
    td.textContent = currentScope === "friends"
      ? "No friends leaderboard entries yet. Add friends in the Manage Friends window."
      : "No leaderboard entries yet.";
    tr.appendChild(td);
    tbody.appendChild(tr);
  }

  rows.forEach((row, index) => {
    const tr = document.createElement("tr");

    const rankTd = document.createElement("td");
    rankTd.textContent = index + 1;
    rankTd.classList.add("rank");

    const userTd = document.createElement("td");
    userTd.appendChild(renderUserCell(row));

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

function openFriendsManager() {
  const width = 520;
  const height = 640;
  const left = Math.max(0, window.screenX + Math.round((window.outerWidth - width) / 2));
  const top = Math.max(0, window.screenY + Math.round((window.outerHeight - height) / 2));
  const features = [
    `width=${width}`,
    `height=${height}`,
    `left=${left}`,
    `top=${top}`,
    "resizable=yes",
    "scrollbars=yes",
  ].join(",");

  const popup = window.open("/friends.html", "pressle-friends", features);
  if (!popup) {
    window.location.href = "/friends.html";
    return;
  }

  popup.focus();
}

function onFriendListUpdatedFromPopup(event) {
  if (event.origin !== window.location.origin) return;
  if (!event.data || !event.data.type) return;

  if (event.data.type === "friends-updated") {
    if (currentScope === "friends") {
      refreshLeaderboard();
    }
    return;
  }

  if (event.data.type === "profile-updated") {
    refreshLeaderboard();
    return;
  }

  if (event.data.type === "self-profile-deleted") {
    window.location.href = "/login.html";
  }
}

// Initialization (runs once after the HTML loads)
document.addEventListener("DOMContentLoaded", () => {
  const scopeSelect = document.getElementById("scope-select");
  const windowSelect = document.getElementById("window-select");
  const logoutBtn = document.getElementById("logout-btn");
  const manageFriendsBtn = document.getElementById("manage-friends-btn");

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

  if (logoutBtn) {
    logoutBtn.addEventListener("click", logout);
  }

  if (manageFriendsBtn) {
    manageFriendsBtn.addEventListener("click", openFriendsManager);
  }

  window.addEventListener("message", onFriendListUpdatedFromPopup);

  refreshAuthUI().then((ok) => {
    if (ok) refreshLeaderboard();
  });
});
