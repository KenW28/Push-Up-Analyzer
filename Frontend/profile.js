function setProfileStatus(message, isError) {
  const statusEl = document.getElementById("profile-status");
  if (!statusEl) return;

  statusEl.textContent = message;
  statusEl.style.color = isError ? "#f7a8a8" : "#c9bfb6";
}

function formatDate(isoValue) {
  if (!isoValue) return "—";
  const d = new Date(isoValue);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString();
}

function notifyOpener(type) {
  if (!window.opener || window.opener.closed) return;
  window.opener.postMessage({ type }, window.location.origin);
}

function closeProfileWindow() {
  window.close();
  if (!window.closed && window.history.length > 1) {
    window.history.back();
  }
}

function getTargetUsernameFromURL() {
  const params = new URLSearchParams(window.location.search);
  return String(params.get("username") || "").trim();
}

function fetchAuthState() {
  return fetch("/api/auth/me", { headers: { Accept: "application/json" } })
    .then((res) => {
      if (!res.ok) throw new Error("Failed to check auth state");
      return res.json();
    })
    .then((payload) => {
      if (!payload.loggedIn) {
        window.location.href = "/login.html";
        return null;
      }
      return payload;
    });
}

function fetchProfile(username) {
  return fetch(`/api/profiles/${encodeURIComponent(username)}`, {
    method: "GET",
    headers: { Accept: "application/json" },
  }).then(async (res) => {
    if (res.status === 401) {
      window.location.href = "/login.html";
      throw new Error("Unauthorized");
    }
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `Failed to load profile (${res.status})`);
    }
    return res.json();
  });
}

function renderProfile(profile) {
  const titleEl = document.getElementById("profile-title");
  const subtitleEl = document.getElementById("profile-subtitle");

  if (titleEl) titleEl.textContent = `${profile.username}'s Profile`;
  if (subtitleEl) {
    subtitleEl.textContent = profile.isSelf
      ? "This is your profile."
      : "Viewing public profile information.";
  }

  document.getElementById("profile-username").textContent = profile.username;
  document.getElementById("profile-created-at").textContent = formatDate(profile.createdAt);
  document.getElementById("profile-total-reps").textContent = String(profile.totalReps || 0);
  document.getElementById("profile-streak").textContent = `${profile.streakDays || 0} day(s)`;
  document.getElementById("profile-friends-count").textContent = String(profile.friendsCount || 0);
  document.getElementById("profile-founder").textContent = profile.isFounder ? "Founder" : "No";

  const deleteBtn = document.getElementById("delete-profile-btn");
  if (deleteBtn) {
    deleteBtn.hidden = !profile.isSelf;
  }
}

function deleteOwnProfile() {
  const deleteBtn = document.getElementById("delete-profile-btn");

  const firstConfirm = window.confirm("Delete your profile? This permanently removes your account and all associated data.");
  if (!firstConfirm) return;

  const secondConfirm = window.confirm("This cannot be undone. Continue deleting your profile?");
  if (!secondConfirm) return;

  if (deleteBtn) deleteBtn.disabled = true;
  setProfileStatus("Deleting profile...", false);

  fetch("/api/profiles/me", {
    method: "DELETE",
    headers: { Accept: "application/json" },
  })
    .then(async (res) => {
      if (res.status === 401) {
        window.location.href = "/login.html";
        throw new Error("Unauthorized");
      }
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `Delete failed (${res.status})`);
      }

      notifyOpener("profile-updated");
      notifyOpener("self-profile-deleted");
      setProfileStatus("Profile deleted. Redirecting to login...", false);

      setTimeout(() => {
        closeProfileWindow();
        if (!window.closed) {
          window.location.href = "/login.html";
        }
      }, 350);
      return null;
    })
    .catch((err) => {
      console.error(err);
      setProfileStatus(err.message || "Failed to delete profile.", true);
      if (deleteBtn) deleteBtn.disabled = false;
    });
}

document.addEventListener("DOMContentLoaded", () => {
  const closeBtn = document.getElementById("close-profile-btn");
  const deleteBtn = document.getElementById("delete-profile-btn");

  if (closeBtn) {
    closeBtn.addEventListener("click", closeProfileWindow);
  }

  if (deleteBtn) {
    deleteBtn.addEventListener("click", deleteOwnProfile);
  }

  fetchAuthState()
    .then((auth) => {
      if (!auth) return null;

      const requestedUsername = getTargetUsernameFromURL();
      const username = requestedUsername || auth.username;
      if (!username) throw new Error("No username specified");

      return fetchProfile(username)
        .then((profile) => {
          renderProfile(profile);
          setProfileStatus("Profile loaded.", false);
        });
    })
    .catch((err) => {
      console.error(err);
      setProfileStatus(err.message || "Failed to load profile.", true);
    });
});
