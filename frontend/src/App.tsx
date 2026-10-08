import { useCallback, useEffect, useState } from "react";
import { Link, NavLink, Route, Routes, useLocation } from "react-router-dom";
import { refresh, setAuthChangeHandler } from "./api";
import Home from "./Home";
import ListPage from "./ListPage";
import Login from "./Login";
import PageView from "./PageView";
import Profile from "./Profile";

export default function App() {
  // "unavailable": the session could not be checked (offline, server down).
  // That is not a logout, so the login form is not shown for it.
  const [session, setSession] = useState<"loading" | "in" | "out" | "unavailable">("loading");

  const [menuOpen, setMenuOpen] = useState(false);
  const { pathname } = useLocation();

  // Close the side nav after navigating, or on Escape.
  useEffect(() => setMenuOpen(false), [pathname]);
  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setMenuOpen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [menuOpen]);

  const restore = useCallback(() => {
    setSession("loading");
    refresh().then((r) =>
      // A broadcast from another tab may already have signed this one in.
      setSession((cur) => (cur !== "loading" ? cur : r === "ok" ? "in" : r === "expired" ? "out" : "unavailable")),
    );
  }, []);

  useEffect(() => {
    setAuthChangeHandler(setSession);
    restore();
  }, [restore]);

  // The server was unreachable at startup: try again once the network is back.
  useEffect(() => {
    if (session !== "unavailable") return;
    window.addEventListener("online", restore);
    return () => window.removeEventListener("online", restore);
  }, [session, restore]);

  if (session === "loading") return <main className="page muted">Loading…</main>;
  if (session === "unavailable") {
    return (
      <main className="page narrow">
        <h1>Lists</h1>
        <p>Can't reach the server. You are still signed in.</p>
        <div>
          <button className="primary" onClick={restore}>
            Try again
          </button>
        </div>
      </main>
    );
  }
  if (session === "out") return <Login onLogin={() => setSession("in")} />;

  return (
    <>
      <header className="bar">
        <div>
          <Link to="/" className="brand">
            Lists
          </Link>
          <button aria-label="Menu" aria-expanded={menuOpen} aria-controls="sidenav" onClick={() => setMenuOpen((o) => !o)}>
            {menuOpen ? "✕" : "☰"}
          </button>
        </div>
      </header>
      {menuOpen && <div className="backdrop" onClick={() => setMenuOpen(false)} />}
      <nav id="sidenav" className="sidenav" aria-label="Main" hidden={!menuOpen}>
        <NavLink to="/" end>
          Lists
        </NavLink>
        <NavLink to="/profile">Profile</NavLink>
      </nav>
      <main className="page">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/lists/:id" element={<ListPage />} />
          <Route path="/pages/:id" element={<PageView />} />
          <Route path="/profile" element={<Profile onLogout={() => setSession("out")} />} />
          <Route path="*" element={<p>Not found.</p>} />
        </Routes>
      </main>
    </>
  );
}
