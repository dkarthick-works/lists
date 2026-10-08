import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError, lists, type PageDetail } from "./api";
import { DoubleTapButton, InlineEdit } from "./ListPage";

// A page: the full text behind an entry's short title. It has no entries of
// its own; the breadcrumb leads back to the list it belongs to.
export default function PageView() {
  const { id = "" } = useParams();
  const [data, setData] = useState<PageDetail | null>(null);
  const [error, setError] = useState("");
  const [renaming, setRenaming] = useState(false);
  // null unless the text is being edited.
  const [draft, setDraft] = useState<string | null>(null);

  const load = useCallback(() => lists.getPage(id).then(setData), [id]);

  useEffect(() => {
    setData(null);
    setDraft(null);
    setRenaming(false);
    setError("");
    load().catch((e) => setError(e instanceof ApiError && e.status === 404 ? "Page not found." : e.message));
  }, [load]);

  if (!data) return <p className={error ? undefined : "muted"}>{error || "Loading…"}</p>;
  const { page, ancestors } = data;

  const save = (action: () => Promise<unknown>) =>
    action()
      .then(load)
      .then(() => {
        setError("");
        setDraft(null);
      })
      .catch((e) => setError(e.message));

  return (
    <>
      <nav className="crumbs" aria-label="Breadcrumb">
        {ancestors.map((a, i) => (
          <span key={a.id}>
            {i > 0 && " / "}
            <Link to={`/lists/${a.id}`}>{a.text}</Link>
          </span>
        ))}
      </nav>

      <div className="title">
        {renaming ? (
          <InlineEdit
            label="Page title"
            value={page.text}
            onSave={(text) => save(() => lists.rename(page.id, text))}
            onClose={() => setRenaming(false)}
          />
        ) : (
          <h1>
            <DoubleTapButton className="rename" title="Double-tap to rename" onDoubleTap={() => setRenaming(true)}>
              {page.text}
            </DoubleTapButton>
          </h1>
        )}
      </div>
      {error && <p role="alert">{error}</p>}

      {draft === null ? (
        <>
          <p className="page-body">{page.body}</p>
          <div>
            <button onClick={() => setDraft(page.body ?? "")}>Edit</button>
          </div>
        </>
      ) : (
        <form
          className="stack page-edit"
          onSubmit={(e) => {
            e.preventDefault();
            if (draft.trim()) save(() => lists.setBody(page.id, draft));
          }}
        >
          <textarea autoFocus aria-label="Page text" value={draft} onChange={(e) => setDraft(e.target.value)} />
          <div className="confirm-actions">
            <button type="submit" className="primary" disabled={!draft.trim()}>
              Save
            </button>
            <button type="button" onClick={() => setDraft(null)}>
              Cancel
            </button>
          </div>
        </form>
      )}
    </>
  );
}
