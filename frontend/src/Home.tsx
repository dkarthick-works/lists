import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Composer from "./Composer";
import SearchBox from "./SearchBox";
import { lists, type Item } from "./api";

function ListLinks({ items, empty }: { items: Item[]; empty: string }) {
  if (items.length === 0) return <p className="muted">{empty}</p>;
  return (
    <ul className="rows">
      {items.map((l) => (
        <li key={l.id}>
          <Link to={`/lists/${l.id}`} className="row-link">
            {l.text}
          </Link>
        </li>
      ))}
    </ul>
  );
}

export default function Home() {
  const [recent, setRecent] = useState<Item[] | null>(null);
  const [all, setAll] = useState<Item[] | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(
    () =>
      Promise.all([lists.recent(), lists.all()]).then(([r, a]) => {
        setRecent(r);
        setAll(a);
      }),
    [],
  );

  useEffect(() => {
    load().catch((e) => setError(e.message));
  }, [load]);

  // The new list shows up under "Recently edited"; opening it is a separate tap.
  async function create(title: string) {
    try {
      await lists.create(title);
      await load();
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create list");
      throw err;
    }
  }

  return (
    <>
      <SearchBox />
      {error && <p role="alert">{error}</p>}

      <section>
        <h2>Recently edited</h2>
        {recent ? <ListLinks items={recent} empty="No lists yet." /> : <p className="muted">Loading…</p>}
      </section>
      {all && all.length > 5 && (
        <section>
          <h2>All lists</h2>
          <ListLinks items={all} empty="" />
        </section>
      )}

      <Composer placeholder="New list name" onSend={create} />
    </>
  );
}
