import { useEffect, useRef, useState, type FormEvent } from "react";

// Chat-style input pinned to the bottom of the screen. The field clears once
// onSend resolves. It takes long text too: the server turns that into a page.
export default function Composer({
  placeholder,
  onSend,
}: {
  placeholder: string;
  onSend: (text: string) => Promise<unknown>;
}) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);

  // Grow with the text (up to the CSS max-height), and shrink back when cleared.
  const field = useRef<HTMLTextAreaElement>(null);
  useEffect(() => {
    const el = field.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${el.scrollHeight + 2}px`;
  }, [text]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!text.trim() || busy) return;
    setBusy(true);
    try {
      await onSend(text);
      setText("");
    } catch {
      // The caller reports the error; keep the text so it can be retried.
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="composer">
      <div>
        <textarea
          ref={field}
          rows={1}
          placeholder={placeholder}
          aria-label={placeholder}
          enterKeyHint="send"
          value={text}
          onChange={(e) => setText(e.target.value)}
          // Enter sends, like a chat box; Shift+Enter adds a line.
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              e.currentTarget.form?.requestSubmit();
            }
          }}
        />
        <button type="submit" className="primary" aria-label="Send" disabled={!text.trim() || busy}>
          ↑
        </button>
      </div>
    </form>
  );
}

// A new page starts with a trimmed title; the server swaps in a generated one
// a moment later. This returns a function that re-runs `load` a couple of
// times to pick that up, and drops the pending runs when `load` changes
// (another list was opened) or the screen goes away.
export function useRefreshSoon(load: () => Promise<unknown>) {
  const timers = useRef<number[]>([]);
  useEffect(() => () => timers.current.forEach(clearTimeout), [load]);
  return () => {
    timers.current = [1500, 5000].map((ms) => window.setTimeout(() => load().catch(() => {}), ms));
  };
}
