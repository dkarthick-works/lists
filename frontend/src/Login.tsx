import { useState, type FormEvent } from "react";
import { forgotPassword, login, signup } from "./api";

type Mode = "login" | "signup" | "forgot";

const titles: Record<Mode, string> = {
  login: "Log in",
  signup: "Sign up",
  forgot: "Reset password",
};

export default function Login({ onLogin }: { onLogin: () => void }) {
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  function switchTo(next: Mode) {
    setMode(next);
    setError("");
    setNotice("");
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    try {
      if (mode === "login") {
        await login(email, password);
        onLogin();
        return;
      }
      const res = mode === "signup" ? await signup(email, password) : await forgotPassword(email);
      setNotice(res.message);
      setPassword("");
      setMode("login");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="page narrow">
      <h1>Lists</h1>
      <form onSubmit={submit} className="stack">
        <h2>{titles[mode]}</h2>
        <label>
          Email
          <input
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>
        {mode !== "forgot" && (
          <label>
            Password
            <input
              type="password"
              autoComplete={mode === "login" ? "current-password" : "new-password"}
              required
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>
        )}
        {error && <p role="alert">{error}</p>}
        {notice && <p role="status">{notice}</p>}
        <button type="submit" className="primary" disabled={busy}>
          {titles[mode]}
        </button>
      </form>
      <p className="links">
        {mode !== "login" && (
          <button className="link" onClick={() => switchTo("login")}>
            Log in
          </button>
        )}
        {mode !== "signup" && (
          <button className="link" onClick={() => switchTo("signup")}>
            Create an account
          </button>
        )}
        {mode !== "forgot" && (
          <button className="link" onClick={() => switchTo("forgot")}>
            Forgot password
          </button>
        )}
      </p>
    </main>
  );
}
