import { useEffect, useState } from "react";
import { Link, NavLink, Route, Routes, useLocation } from "react-router-dom";
import { refresh, setSignedOutHandler } from "./api";
import Home from "./Home";
import ListPage from "./ListPage";
import Login from "./Login";
import Profile from "./Profile";

export default function App() {
  // null while the stored session is being restored.
  const [signedIn, setSignedIn] = useState<boolean | null>(null);

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

  useEffect(() => {
    setSignedOutHandler(() => setSignedIn(false));
    refresh().then(setSignedIn);
  }, []);

  if (signedIn === null) return <main className="page muted">Loading…</main>;
  if (!signedIn) return <Login onLogin={() => setSignedIn(true)} />;

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
          <Route path="/profile" element={<Profile onLogout={() => setSignedIn(false)} />} />
          <Route path="*" element={<p>Not found.</p>} />
        </Routes>
      </main>
    </>
  );
}
