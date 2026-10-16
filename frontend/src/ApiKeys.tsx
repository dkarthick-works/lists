import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, createKey, listKeys, revokeKey, type ApiKey } from "./api";

const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 31536000],
  ["month", 2592000],
  ["day", 86400],
  ["hour", 3600],
  ["minute", 60],
];

function ago(iso: string) {
  const seconds = (Date.now() - Date.parse(iso)) / 1000;
  for (const [unit, size] of UNITS) if (seconds >= size) return rtf.format(-Math.floor(seconds / size), unit);
  return "just now";
}

// Copies text, then says so on the button for a moment. Without a clipboard
// (outside a secure context) the text it belongs to selects in one tap instead.
function CopyButton({ text, label }: { text: string; label: string }) {
  const [status, setStatus] = useState<"" | "Copied" | "Couldn't copy">("");

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setStatus("Copied");
    } catch {
      setStatus("Couldn't copy");
    }
    setTimeout(() => setStatus(""), 2000);
  }

  return (
    <button onClick={copy} aria-label={`Copy ${label}`}>
      <span aria-live="polite">{status || "Copy"}</span>
    </button>
  );
}

// A read-only block of text with a Copy button under it.
function Copyable({ label, text }: { label: string; text: string }) {
  return (
    <div className="stack">
      <pre className="secret" aria-label={label}>
        {text}
      </pre>
      <div className="copy-row">
        <CopyButton text={text} label={label} />
      </div>
    </div>
  );
}

export default function ApiKeys() {
  const [keys, setKeys] = useState<ApiKey[] | null>(null);
  const [error, setError] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  // The raw key from the last create. Dismissing it drops it for good.
  const [created, setCreated] = useState<string | null>(null);
  // Id of the key whose revoke is waiting for confirmation.
  const [confirming, setConfirming] = useState<string | null>(null);

  const load = useCallback(() => listKeys().then(setKeys), []);

  useEffect(() => {
    load().catch((e) => setError(e.message));
  }, [load]);

  async function create(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const k = await createKey(name);
      setCreated(k.key);
      setName("");
      setError("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create key");
    } finally {
      setBusy(false);
    }
  }

  function revoke(id: string) {
    setConfirming(null);
    revokeKey(id)
      // Already gone (revoked in another tab): the reload shows that.
      .catch((e) => {
        if (!(e instanceof ApiError && e.status === 404)) throw e;
      })
      .then(load)
      .then(() => setError(""))
      .catch((e) => setError(e.message));
  }

  const endpoint = `${location.origin}/mcp`;
  const command =
    created && `claude mcp add --transport http lists ${endpoint} --header "Authorization: Bearer ${created}"`;

  return (
    <section className="stack">
      <h2>Developer settings</h2>
      <p className="measure">API keys let AI agents (Claude Code, Cursor, …) read and add to your lists over MCP.</p>

      <ul className="rows">
        <li className="key-row">
          <div className="row-text">
            <span className="small muted">MCP endpoint</span>
            <div className="secret-inline">
              {/* On a narrow screen, wrap before /mcp rather than inside a word. */}
              {location.origin}
              <wbr />
              /mcp
            </div>
          </div>
          <CopyButton text={endpoint} label="MCP endpoint" />
        </li>
      </ul>

      <div className="stack group">
        <span className="field-label">{keys && keys.length > 0 ? `Keys (${keys.length})` : "Keys"}</span>
        {keys === null ? (
          <p className="muted">Loading…</p>
        ) : keys.length === 0 ? (
          <p className="muted">No API keys yet.</p>
        ) : (
          <ul className="rows">
            {keys.map((k) =>
              confirming === k.id ? (
                <li key={k.id} className="key-confirm stack">
                  <p>Revoke "{k.name}"? Agents using it will stop working.</p>
                  <div className="confirm-actions">
                    <button className="primary" onClick={() => revoke(k.id)}>
                      Revoke
                    </button>
                    <button onClick={() => setConfirming(null)}>Cancel</button>
                  </div>
                </li>
              ) : (
                <li key={k.id} className="key-row">
                  <div className="row-text">
                    <strong>{k.name}</strong>
                    <div className="small muted">
                      Created {new Date(k.created_at).toLocaleDateString()} · Last used{" "}
                      {k.last_used_at ? ago(k.last_used_at) : "never"}
                    </div>
                  </div>
                  <button onClick={() => setConfirming(k.id)}>Revoke</button>
                </li>
              ),
            )}
          </ul>
        )}
      </div>

      <form className="inline-form group" onSubmit={create}>
        <label>
          New key name
          <input value={name} maxLength={100} placeholder="e.g. Laptop" onChange={(e) => setName(e.target.value)} />
        </label>
        <button type="submit" className="primary" disabled={busy || !name.trim()}>
          Create
        </button>
      </form>
      {error && <p role="alert">{error}</p>}

      {created && command && (
        <div className="stack reveal">
          <p>
            <strong>Copy your new key now. You won't see it again.</strong>
          </p>
          <Copyable label="New API key" text={created} />
          <p>To connect Claude Code, run:</p>
          <Copyable label="Claude Code command" text={command} />
          <div>
            <button onClick={() => setCreated(null)}>Done</button>
          </div>
        </div>
      )}
    </section>
  );
}
