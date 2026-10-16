import { useEffect, useState } from "react";
import ApiKeys from "./ApiKeys";
import { logout, me } from "./api";

export default function Profile({ onLogout }: { onLogout: () => void }) {
  const [email, setEmail] = useState("");

  useEffect(() => {
    me()
      .then((u) => setEmail(u.email))
      .catch(() => {});
  }, []);

  return (
    <>
      <h1>Profile</h1>
      <div className="settings">
        <section className="stack">
          <h2>Account</h2>
          <ul className="rows">
            <li className="key-row">
              <div className="row-text">
                <span className="small muted">Signed in as</span>
                <div className={email ? undefined : "muted"}>{email || "Loading…"}</div>
              </div>
              <button
                onClick={async () => {
                  await logout();
                  onLogout();
                }}
              >
                Log out
              </button>
            </li>
          </ul>
        </section>
        <ApiKeys />
      </div>
    </>
  );
}
