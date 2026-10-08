import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import Composer, { useRefreshSoon } from "./Composer";
import { ApiError, lists, type Item, type ListDetail } from "./api";

// Router state that puts a list page into its delete confirmation.
const CONFIRM_DELETE = { confirmDelete: true };

function TrashIcon() {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M4 6.5h16M9 6.5v-3h6v3M6 6.5l1 14h10l1-14M10 10.5v6M14 10.5v6" strokeLinejoin="miter" />
    </svg>
  );
}

// A button that only acts on two taps within 300ms, so a stray tap does
// nothing. Timed by hand because touch browsers do not all fire dblclick.
// Keyboard activation (e.detail === 0) is deliberate, so it acts at once.
export function DoubleTapButton({
  onDoubleTap,
  ...props
}: { onDoubleTap: () => void } & Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, "onClick">) {
  const lastTap = useRef(0);
  return (
    <button
      {...props}
      onClick={(e) => {
        const now = Date.now();
        const double = now - lastTap.current < 300;
        lastTap.current = now;
        if (double || e.detail === 0) onDoubleTap();
      }}
    />
  );
}

function ListIcon() {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M9 6.5h11M9 12h11M9 17.5h11M4 5.5h2v2H4zM4 11h2v2H4zM4 16.5h2v2H4z" />
    </svg>
  );
}

// Text field for editing a value in place. Enter or leaving the field saves,
// Escape cancels; either way onClose runs first.
export function InlineEdit({
  value,
  label,
  className,
  onSave,
  onClose,
}: {
  value: string;
  label: string;
  className?: string;
  onSave: (text: string) => void;
  onClose: () => void;
}) {
  const [draft, setDraft] = useState(value);
  // Removing the field on Escape can still fire blur; this stops that saving.
  const cancelled = useRef(false);
  return (
    <input
      className={className}
      autoFocus
      aria-label={label}
      maxLength={500}
      enterKeyHint="done"
      value={draft}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => {
        const text = draft.trim();
        onClose();
        if (!cancelled.current && text && text !== value) onSave(text);
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter") e.currentTarget.blur();
        if (e.key === "Escape") {
          cancelled.current = true;
          onClose();
        }
      }}
    />
  );
}

// A plain entry: tap to edit in place.
function EntryRow({ entry, onChange }: { entry: Item; onChange: (action: () => Promise<unknown>) => void }) {
  const [editing, setEditing] = useState(false);
  if (!editing) {
    return (
      <button className="row-text" onClick={() => setEditing(true)}>
        {entry.text}
      </button>
    );
  }
  return (
    <InlineEdit
      className="row-input"
      label="Entry"
      value={entry.text}
      onSave={(text) => onChange(() => lists.rename(entry.id, text))}
      onClose={() => setEditing(false)}
    />
  );
}

export default function ListPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const confirmingDelete = useLocation().state?.confirmDelete === true;
  const [data, setData] = useState<ListDetail | null>(null);
  const [error, setError] = useState("");
  const [renaming, setRenaming] = useState(false);
  // Id of the entry whose ⋯ menu is open.
  const [menuFor, setMenuFor] = useState<string | null>(null);

  const load = useCallback(() => lists.get(id).then(setData), [id]);
  const refreshSoon = useRefreshSoon(load);

  useEffect(() => {
    setData(null);
    setRenaming(false);
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

  // Completed entries sit after the open ones and are not reorderable.
  const openCount = entries.filter((e) => e.completed_at === null).length;

  // Swaps an open entry with its neighbour. The page updates at once and the menu
  // stays open on the moved row, so it can be tapped repeatedly; if saving
  // fails, the list is re-read to show the real order.
  function move(index: number, by: -1 | 1) {
    const target = index + by;
    if (target < 0 || target >= openCount) return;
    const next = [...entries];
    [next[index], next[target]] = [next[target], next[index]];
    setData({ list, ancestors, entries: next });
    lists
      .reorder(
        list.id,
        next.map((e) => e.id),
      )
      .then(() => setError(""))
      .catch((e) => {
        setError(e.message);
        return load();
      })
      .catch(() => {});
  }

  function removeEntry(entry: Item) {
    setMenuFor(null);
    // A sub-list gets the same full-page confirmation as any list.
    if (entry.is_list) navigate(`/lists/${entry.id}`, { state: CONFIRM_DELETE });
    else mutate(() => lists.remove(entry.id));
  }

  // Deleting returns to the parent list, or home for a top-level list.
  function removeList() {
    const parent = ancestors[ancestors.length - 1];
    lists
      .remove(list.id)
      .then(() => navigate(parent ? `/lists/${parent.id}` : "/", { replace: true }))
      .catch((e) => setError(e.message));
  }

  // The confirmation takes over the page; it is a history entry, so Back cancels.
  if (confirmingDelete) {
    const nested = entries.filter((e) => e.is_list);
    const plain = entries.length - nested.length;
    return (
      <>
        <h1>Delete "{list.text}"?</h1>
        <p>This deletes the list and everything in it. It cannot be undone.</p>
        {error && <p role="alert">{error}</p>}
        {nested.length > 0 && (
          <section>
            <h2>Lists inside that will be deleted</h2>
            <ul className="rows">
              {nested.map((e) => (
                <li key={e.id}>
                  <span className="row-text">{e.text}</span>
                </li>
              ))}
            </ul>
          </section>
        )}
        {plain > 0 && (
          <p className="muted">
            {nested.length > 0 ? "Plus " : ""}
            {plain} {plain === 1 ? "entry" : "entries"}.
          </p>
        )}
        <div className="confirm-actions">
          <button className="primary" onClick={removeList}>
            Delete
          </button>
          <button onClick={() => navigate(-1)}>Cancel</button>
        </div>
      </>
    );
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
        {renaming ? (
          <InlineEdit
            label="List name"
            value={list.text}
            onSave={(name) => mutate(() => lists.rename(list.id, name))}
            onClose={() => setRenaming(false)}
          />
        ) : (
          <>
            <h1>
              <DoubleTapButton className="rename" title="Double-tap to rename" onDoubleTap={() => setRenaming(true)}>
                {list.text}
              </DoubleTapButton>
            </h1>
            <DoubleTapButton
              className="icon"
              aria-label="Delete list"
              title="Double-tap to delete list"
              onDoubleTap={() => navigate(`/lists/${list.id}`, { state: CONFIRM_DELETE })}
            >
              <TrashIcon />
            </DoubleTapButton>
          </>
        )}
      </div>
      {error && <p role="alert">{error}</p>}

      {entries.length > 0 && (
        <ul className="rows">
          {entries.map((entry, i) => (
            <li key={entry.id} className={entry.completed_at ? "entry completed" : "entry"}>
              {entry.is_list ? (
                <Link to={`/lists/${entry.id}`} className="row-link">
                  {entry.text} <span aria-hidden="true">→</span>
                </Link>
              ) : entry.body !== null ? (
                <Link to={`/pages/${entry.id}`} className="row-link">
                  {entry.text} <span aria-hidden="true">¶</span>
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
                    <button
                      role="menuitem"
                      className={entry.completed_at ? "icon on" : "icon"}
                      aria-pressed={entry.completed_at !== null}
                      aria-label={entry.completed_at ? "Mark as not completed" : "Mark as completed"}
                      title={entry.completed_at ? "Mark as not completed" : "Mark as completed"}
                      onClick={() => {
                        setMenuFor(null);
                        mutate(() => lists.setCompleted(entry.id, entry.completed_at === null));
                      }}
                    >
                      ✓
                    </button>
                    <button
                      role="menuitem"
                      className="icon"
                      aria-label="Move up"
                      title="Move up"
                      disabled={entry.completed_at !== null || i === 0}
                      onClick={() => move(i, -1)}
                    >
                      ↑
                    </button>
                    <button
                      role="menuitem"
                      className="icon"
                      aria-label="Move down"
                      title="Move down"
                      disabled={entry.completed_at !== null || i >= openCount - 1}
                      onClick={() => move(i, 1)}
                    >
                      ↓
                    </button>
                    {!entry.is_list && entry.body === null && (
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
                    <DoubleTapButton
                      role="menuitem"
                      className="icon"
                      aria-label="Delete"
                      title="Double-tap to delete"
                      onDoubleTap={() => removeEntry(entry)}
                    >
                      <TrashIcon />
                    </DoubleTapButton>
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
            .then((entry) => {
              if (entry.body !== null) refreshSoon();
              return load();
            })
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
