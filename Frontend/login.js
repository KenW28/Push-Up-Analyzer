// Sends username/password to the backend.
// If login succeeds, the backend will set a session cookie.
// Then we redirect to the homepage.
document.addEventListener("DOMContentLoaded", () => {
  const form = document.getElementById("login-form");
  const status = document.getElementById("status");

  if (!form || !status) return;

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    status.textContent = "Logging in...";

    const username = document.getElementById("username")?.value.trim() || "";
    const password = document.getElementById("password")?.value || "";

    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });

      if (!res.ok) {
        const msg = await res.text();
        status.textContent = msg || "Login failed.";
        return;
      }

      window.location.href = "/";
    } catch (err) {
      console.error(err);
      status.textContent = "Network error.";
    }
  });
});
