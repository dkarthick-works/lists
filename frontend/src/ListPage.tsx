import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import Composer from "./Composer";
import { ApiError, lists, type Item, type ListDetail } from "./api";

function TrashIcon() {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M4 6.5h16M9 6.5v-3h6v3M6 6.5l1 14h10l1-14M10 10.5v6M14 10.5v6" strokeLinejoin="miter" />
    </svg>
  );
}

function ListIcon() {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M9 6.5h11M9 12h11M9 17.5h11M4 5.5h2v2H4zM4 11h2v2H4zM4 16.5h2v2H4z" />
    </svg>
  );
}

// A plain entry: tap to edit in place. Enter or leaving the field saves,
// Escape cancels.
function EntryRow({ entry, onChange }: { entry: Item; onChange: (action: () => Promise<unknown>) => void }) {
  const [draft, setDraft] = useState<string | null>(null);
  const cancelled = useRef(false);

  if (draft === null) {
    return (
      <button
        className="row-text"
        onClick={() => {
          cancelled.current = false;
          setDraft(entry.text);
        }}
      >
        {entry.text}
      </button>
    );
  }

  function save() {
    const text = draft?.trim();
    setDraft(null);
    if (cancelled.current || !text || text === entry.text) return;
    onChange(() => lists.rename(entry.id, text));
  }

  return (
    <input
      className="row-input"
      autoFocus
      aria-label="Entry"
      maxLength={500}
      enterKeyHint="done"
      value={draft}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={save}
      onKeyDown={(e) => {
        if (e.key === "Enter") e.currentTarget.blur();
        if (e.key === "Escape") {
          cancelled.current = true;
          setDraft(null);
        }
      }}
    />
  );
}

export default function ListPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [data, setData] = useState<ListDetail | null>(null);
  const [error, setError] = useState("");
  // null unless the list name is being edited.
  const [draft, setDraft] = useState<string | null>(null);
  const cancelled = useRef(false);
  // Id of the entry whose ⋯ menu is open.
  const [menuFor, setMenuFor] = useState<string | null>(null);

  const load = useCallback(() => lists.get(id).then(setData), [id]);

  useEffect(() => {
    setData(null);
    setDraft(null);
    setMenuFor(null);
    setError("");
    load().catch((e) => setError(e instanceof ApiError && e.status === 404 ? "List not found." : e.message));
  }, [load]);

  if (!data) return <p className={error ? undefined : "muted"}>{error || "Loading…"}</p>;
  const { list, entries, ancestors } = data;

  // Runs a change, then re-reads the list so the page shows server state.
  const mutate = (action: () => Promise<unknown>) => {
    action()
      .then(load)
      .then(() => setError(""))
      .catch((e) => setError(e.message));
  };

  function removeEntry(entry: Item) {
    setMenuFor(null);
    if (entry.is_list && !confirm(`Delete "${entry.text}" and everything in it?`)) return;
    mutate(() => lists.remove(entry.id));
  }

  // Tapping the name edits it in place; leaving the field or Enter saves.
  function saveName() {
    const name = draft?.trim();
    setDraft(null);
    if (cancelled.current) return;
    if (!name || name === list.text) return;
    mutate(() => lists.rename(list.id, name));
  }

  // Deleting returns to the parent list, or home for a top-level list.
  function removeList() {
    if (!confirm(`Delete "${list.text}" and everything in it?`)) return;
    const parent = ancestors[ancestors.length - 1];
    lists
      .remove(list.id)
      .then(() => navigate(parent ? `/lists/${parent.id}` : "/"))
      .catch((e) => setError(e.message));
  }

  return (
    <>
      {ancestors.length > 0 && (
        <nav className="crumbs" aria-label="Breadcrumb">
          {ancestors.map((a, i) => (
            <span key={a.id}>
              {i > 0 && " / "}
              <Link to={`/lists/${a.id}`}>{a.text}</Link>
            </span>
          ))}
        </nav>
      )}

      <div className="title">
        {draft === null ? (
          <>
            <h1>
              <button className="rename" title="Rename" onClick={() => {
                  cancelled.current = false;
                  setDraft(list.text);
                }}>
                {list.text}
              </button>
            </h1>
            <button className="icon" aria-label="Delete list" title="Delete list" onClick={removeList}>
              <TrashIcon />
            </button>
          </>
        ) : (
          <input
            autoFocus
            aria-label="List name"
            maxLength={500}
            enterKeyHint="done"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={saveName}
            onKeyDown={(e) => {
              if (e.key === "Enter") e.currentTarget.blur();
              if (e.key === "Escape") {
                // Removing the field can still fire blur; make sure that doesn't save.
                cancelled.current = true;
                setDraft(null);
              }
            }}
          />
        )}
      </div>
      {error && <p role="alert">{error}</p>}

      {entries.length > 0 && (
        <ul className="rows">
          {entries.map((entry) => (
            <li key={entry.id} className="entry">
              {entry.is_list ? (
                <Link to={`/lists/${entry.id}`} className="row-link">
                  {entry.text} <span aria-hidden="true">→</span>
                </Link>
              ) : (
                <EntryRow entry={entry} onChange={mutate} />
              )}
              <button
                className="icon"
                aria-label={`Options for ${entry.text}`}
                aria-haspopup="menu"
                aria-expanded={menuFor === entry.id}
                onClick={() => setMenuFor(menuFor === entry.id ? null : entry.id)}
              >
                ⋯
              </button>
              {menuFor === entry.id && (
                <>
                  <div className="menu-backdrop" onClick={() => setMenuFor(null)} />
                  <div className="menu" role="menu" onKeyDown={(e) => e.key === "Escape" && setMenuFor(null)}>
                    {!entry.is_list && (
                      <button
                        role="menuitem"
                        className="icon"
                        aria-label="Make this entry a list"
                        title="Make this entry a list"
                        onClick={() => {
                          setMenuFor(null);
                          // The entry becomes a list named after it; open it straight away.
                          lists
                            .makeList(entry.id)
                            .then(() => navigate(`/lists/${entry.id}`))
                            .catch((e) => setError(e.message));
                        }}
                      >
                        <ListIcon />
                      </button>
                    )}
                    <button
                      role="menuitem"
                      className="icon"
                      aria-label="Delete"
                      title="Delete"
                      onClick={() => removeEntry(entry)}
                    >
                      <TrashIcon />
                    </button>
                  </div>
                </>
              )}
            </li>
          ))}
        </ul>
      )}

      <Composer
        placeholder="New entry"
        onSend={(text) =>
          lists
            .addEntry(list.id, text, false)
            .then(load)
            .then(() => setError(""))
            .catch((e) => {
              setError(e.message);
              throw e;
            })
        }
      />
    </>
  );
}
