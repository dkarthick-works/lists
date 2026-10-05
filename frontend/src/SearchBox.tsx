import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { lists, type Suggestion } from "./api";

// Search field with an autocomplete dropdown. Arrow keys move through the
// suggestions, Enter opens the highlighted one (or the best match), Escape closes.
export default function SearchBox() {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  // null while closed or while the first results for a query are loading.
  const [items, setItems] = useState<Suggestion[] | null>(null);
  const [active, setActive] = useState(0);
  const [open, setOpen] = useState(false);

  const q = query.trim();
  useEffect(() => {
    if (!q) {
      setItems(null);
      return;
    }
    let stale = false;
    const timer = setTimeout(() => {
      lists
        .autocomplete(q)
        .then((r) => {
          if (stale) return;
          setItems(r);
          setActive(0);
        })
        .catch(() => !stale && setItems([]));
    }, 150);
    return () => {
      stale = true;
      clearTimeout(timer);
    };
  }, [q]);

  const showing = open && q !== "" && items !== null;

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Escape") setOpen(false);
    if (!showing || items.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((active + 1) % items.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((active - 1 + items.length) % items.length);
    } else if (e.key === "Enter") {
      e.preventDefault();
      navigate(`/lists/${items[active].id}`);
    }
  }

  return (
    <div className="search">
      <input
        type="search"
        role="combobox"
        placeholder="Search lists"
        aria-label="Search lists"
        aria-expanded={showing}
        aria-controls="search-suggestions"
        aria-activedescendant={showing && items.length > 0 ? `suggestion-${active}` : undefined}
        aria-autocomplete="list"
        autoComplete="off"
        enterKeyHint="search"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={onKeyDown}
      />
      {showing && (
        <ul id="search-suggestions" role="listbox" className="suggestions">
          {items.length === 0 && <li className="muted">No lists match.</li>}
          {items.map((s, i) => (
            <li
              key={s.id}
              id={`suggestion-${i}`}
              role="option"
              aria-selected={i === active}
              // mousedown, not click: the field's blur would close the dropdown first.
              onMouseDown={(e) => {
                e.preventDefault();
                navigate(`/lists/${s.id}`);
              }}
              onMouseEnter={() => setActive(i)}
            >
              {s.parent_text && <span className="parent">{s.parent_text} / </span>}
              {s.text}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
