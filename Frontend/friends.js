function setFeedback(message, tone) {
  const feedbackEl = document.getElementById("friends-feedback");
  if (!feedbackEl) return;

  if (!message) {
    feedbackEl.hidden = true;
    feedbackEl.textContent = "";
    feedbackEl.className = "friends-feedback";
    return;
  }

  feedbackEl.hidden = false;
  feedbackEl.textContent = message;
  feedbackEl.className = `friends-feedback ${tone === "error" ? "is-error" : "is-success"}`;
}

function notifyOpenerFriendsUpdated() {
  if (!window.opener || window.opener.closed) return;
  window.opener.postMessage({ type: "friends-updated" }, window.location.origin);
}

function ensureLoggedIn() {
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
      return true;
    })
    .catch((err) => {
      console.error("Auth check failed:", err);
      window.location.href = "/login.html";
      return false;
    });
}

function parseError(res) {
  return res.text().then((text) => {
    if (text && text.trim()) return text.trim();
    return `Request failed (${res.status})`;
  });
}

function apiRequest(url, options) {
  return fetch(url, options).then(async (res) => {
    if (res.status === 401) {
      window.location.href = "/login.html";
      throw new Error("Unauthorized");
    }
    if (!res.ok) {
      throw new Error(await parseError(res));
    }
    return res;
  });
}

function getFriendsState() {
  return apiRequest("/api/friends", {
    method: "GET",
    headers: { Accept: "application/json" },
  }).then((res) => res.json())
    .then((payload) => ({
      friends: Array.isArray(payload?.friends) ? payload.friends : [],
      incomingRequests: Array.isArray(payload?.incomingRequests) ? payload.incomingRequests : [],
      outgoingRequests: Array.isArray(payload?.outgoingRequests) ? payload.outgoingRequests : [],
    }));
}

function sendFriendRequest(username) {
  return apiRequest("/api/friends", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify({ username }),
  });
}

function acceptFriendRequest(requestID) {
  return apiRequest(`/api/friends/requests/${encodeURIComponent(requestID)}/accept`, {
    method: "POST",
    headers: { Accept: "application/json" },
  });
}

function denyFriendRequest(requestID) {
  return apiRequest(`/api/friends/requests/${encodeURIComponent(requestID)}/deny`, {
    method: "POST",
    headers: { Accept: "application/json" },
  });
}

function cancelFriendRequest(requestID) {
  return apiRequest(`/api/friends/requests/${encodeURIComponent(requestID)}`, {
    method: "DELETE",
    headers: { Accept: "application/json" },
  });
}

function removeFriend(username) {
  return apiRequest(`/api/friends/${encodeURIComponent(username)}`, {
    method: "DELETE",
    headers: { Accept: "application/json" },
  });
}

function formatRequestDate(isoDate) {
  const d = new Date(isoDate);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString();
}

function createActionButton(label, className, onClick) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = `friends-item-btn ${className}`;
  button.textContent = label;
  button.addEventListener("click", () => {
    button.disabled = true;
    onClick()
      .catch((err) => {
        console.error(err);
        setFeedback(err.message || "Action failed.", "error");
        button.disabled = false;
      });
  });
  return button;
}

function renderList(items, listID, emptyID, itemRenderer) {
  const listEl = document.getElementById(listID);
  const emptyEl = document.getElementById(emptyID);
  if (!listEl || !emptyEl) return;

  listEl.innerHTML = "";

  if (items.length === 0) {
    emptyEl.style.display = "block";
    return;
  }

  emptyEl.style.display = "none";
  items.forEach((item) => {
    const row = itemRenderer(item);
    if (row) listEl.appendChild(row);
  });
}

function buildListRow(username, metaText, actions) {
  const row = document.createElement("div");
  row.className = "friends-list-item";

  const main = document.createElement("div");
  main.className = "friends-list-main";

  const name = document.createElement("span");
  name.className = "friends-list-name";
  name.textContent = username;
  main.appendChild(name);

  if (metaText) {
    const meta = document.createElement("span");
    meta.className = "friends-list-meta";
    meta.textContent = metaText;
    main.appendChild(meta);
  }

  const actionsWrap = document.createElement("div");
  actionsWrap.className = "friends-item-actions";
  actions.forEach((actionBtn) => actionsWrap.appendChild(actionBtn));

  row.appendChild(main);
  row.appendChild(actionsWrap);
  return row;
}

function renderFriends(friends) {
  renderList(friends, "friends-list", "friends-empty-state", (friend) => {
    const username = String(friend?.username || "").trim();
    if (!username) return null;

    const removeBtn = createActionButton("Remove", "is-danger", () => {
      return removeFriend(username)
        .then(() => {
          setFeedback(`${username} removed from your friends list.`, "success");
          notifyOpenerFriendsUpdated();
          return refreshFriends();
        });
    });

    return buildListRow(username, "Accepted friend", [removeBtn]);
  });
}

function renderIncomingRequests(requests) {
  renderList(requests, "incoming-requests-list", "incoming-empty-state", (request) => {
    const username = String(request?.username || "").trim();
    const requestID = Number(request?.id || 0);
    if (!username || !requestID) return null;

    const metaText = request.createdAt ? `Requested ${formatRequestDate(request.createdAt)}` : "Incoming request";

    const acceptBtn = createActionButton("Accept", "is-success", () => {
      return acceptFriendRequest(requestID)
        .then(() => {
          setFeedback(`${username} is now your friend.`, "success");
          notifyOpenerFriendsUpdated();
          return refreshFriends();
        });
    });

    const denyBtn = createActionButton("Deny", "is-secondary", () => {
      return denyFriendRequest(requestID)
        .then(() => {
          setFeedback(`Denied request from ${username}.`, "success");
          return refreshFriends();
        });
    });

    return buildListRow(username, metaText, [acceptBtn, denyBtn]);
  });
}

function renderOutgoingRequests(requests) {
  renderList(requests, "outgoing-requests-list", "outgoing-empty-state", (request) => {
    const username = String(request?.username || "").trim();
    const requestID = Number(request?.id || 0);
    if (!username || !requestID) return null;

    const metaText = request.createdAt ? `Sent ${formatRequestDate(request.createdAt)}` : "Pending request";

    const cancelBtn = createActionButton("Cancel", "is-secondary", () => {
      return cancelFriendRequest(requestID)
        .then(() => {
          setFeedback(`Cancelled request to ${username}.`, "success");
          return refreshFriends();
        });
    });

    return buildListRow(username, metaText, [cancelBtn]);
  });
}

function refreshFriends() {
  return getFriendsState().then((state) => {
    renderIncomingRequests(state.incomingRequests);
    renderOutgoingRequests(state.outgoingRequests);
    renderFriends(state.friends);
  });
}

document.addEventListener("DOMContentLoaded", () => {
  const form = document.getElementById("friends-form");
  const input = document.getElementById("friend-username-input");
  const closeBtn = document.getElementById("close-window-btn");

  if (closeBtn) {
    closeBtn.addEventListener("click", () => {
      window.close();
    });
  }

  if (form && input) {
    form.addEventListener("submit", (event) => {
      event.preventDefault();
      setFeedback("", "success");

      const username = input.value.trim();
      if (!username) {
        setFeedback("Enter a username.", "error");
        return;
      }

      sendFriendRequest(username)
        .then(() => {
          input.value = "";
          setFeedback(`Friend request sent to ${username}.`, "success");
          return refreshFriends();
        })
        .catch((err) => {
          console.error(err);
          setFeedback(err.message || "Failed to send request.", "error");
        });
    });
  }

  ensureLoggedIn().then((ok) => {
    if (!ok) return;
    refreshFriends().catch((err) => {
      console.error(err);
      setFeedback(err.message || "Failed to load friends.", "error");
    });
  });
});
