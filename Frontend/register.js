// Creates an account on the backend.
// On success, we send the user to login.
document.addEventListener("DOMContentLoaded", () => {
  const form = document.getElementById("register-form");
  const status = document.getElementById("status");

  if (!form || !status) return;

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    status.textContent = "Creating account...";

    const username = document.getElementById("username")?.value.trim() || "";
    const password = document.getElementById("password")?.value || "";

    try {
      const res = await fetch("/api/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });

      if (!res.ok) {
        const msg = await res.text();
        status.textContent = msg || "Registration failed.";
        return;
      }

      status.textContent = "Account created. Redirecting to login...";
      setTimeout(() => {
        window.location.href = "/login.html";
      }, 700);
    } catch (err) {
      console.error(err);
      status.textContent = "Network error.";
    }
  });
});
