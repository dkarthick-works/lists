import { useEffect, useState } from "react";
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
      <section>
        <h2>Signed in as</h2>
        <p className={email ? undefined : "muted"}>{email || "Loading…"}</p>
      </section>
      <div>
        <button
          onClick={async () => {
            await logout();
            onLogout();
          }}
        >
          Log out
        </button>
      </div>
    </>
  );
}
