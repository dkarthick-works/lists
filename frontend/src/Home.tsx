import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Composer, { useRefreshSoon } from "./Composer";
import SearchBox from "./SearchBox";
import { lists, type Item } from "./api";

// Mirrors the server-side limit.
const MAX_PINNED = 5;

function PinIcon({ filled }: { filled: boolean }) {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" fill={filled ? "currentColor" : "none"} stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M9 3.5h6v2l-1 1v4.5l3 3v2H7v-2l3-3V6.5l-1-1zM12 16v5" strokeLinejoin="miter" />
    </svg>
  );
}

// Rows that open a list, each with a pin toggle at the right end.
function ListLinks({
  items,
  empty,
  onPin,
  pinsFull,
}: {
  items: Item[];
  empty: string;
  onPin: (item: Item) => void;
  // True once the pin limit is reached: unpinned rows can no longer be pinned.
  pinsFull: boolean;
}) {
  if (items.length === 0) return <p className="muted">{empty}</p>;
  return (
    <ul className="rows">
      {items.map((l) => (
        <li key={l.id} className="entry">
          <Link to={`/lists/${l.id}`} className="row-link">
            {l.text}
          </Link>
          <button
            className="icon"
            aria-label={l.pinned_at ? `Unpin ${l.text}` : `Pin ${l.text}`}
            aria-pressed={l.pinned_at !== null}
            title={l.pinned_at ? "Unpin" : pinsFull ? `You can pin at most ${MAX_PINNED} lists` : "Pin"}
            disabled={pinsFull && l.pinned_at === null}
            onClick={() => onPin(l)}
          >
            <PinIcon filled={l.pinned_at !== null} />
          </button>
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

  const refreshSoon = useRefreshSoon(load);

  useEffect(() => {
    load().catch((e) => setError(e.message));
  }, [load]);

  // The new list shows up under "Recently edited"; opening it is a separate tap.
  async function create(title: string) {
    try {
      const list = await lists.create(title);
      // A different title back means the text became a page with a placeholder title.
      if (list.text !== title.trim()) refreshSoon();
      await load();
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create list");
      throw err;
    }
  }

  // Oldest pin first, so pinning another list never shuffles the ones above it.
  const pinned = (all ?? []).filter((l) => l.pinned_at !== null).sort((a, b) => a.pinned_at!.localeCompare(b.pinned_at!));

  const pinsFull = pinned.length >= MAX_PINNED;

  function togglePin(item: Item) {
    lists
      .setPinned(item.id, item.pinned_at === null)
      .then(load)
      .then(() => setError(""))
      .catch((e) => setError(e.message));
  }

  return (
    <>
      <SearchBox />
      {error && <p role="alert">{error}</p>}

      {pinned.length > 0 && (
        <section>
          <h2>Pinned</h2>
          <ListLinks items={pinned} empty="" onPin={togglePin} pinsFull={pinsFull} />
        </section>
      )}
      <section>
        <h2>Recently edited</h2>
        {recent ? (
          <ListLinks items={recent} empty={pinned.length > 0 ? "No other lists." : "No lists yet."} onPin={togglePin} pinsFull={pinsFull} />
        ) : (
          <p className="muted">Loading…</p>
        )}
      </section>
      {all && all.length - pinned.length > 5 && (
        <section>
          <h2>All lists</h2>
          <ListLinks items={all} empty="" onPin={togglePin} pinsFull={pinsFull} />
        </section>
      )}

      <Composer placeholder="New list name" onSend={create} />
    </>
  );
}
