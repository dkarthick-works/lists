import { useState, type FormEvent } from "react";

// Chat-style input pinned to the bottom of the screen. The field clears once
// onSend resolves.
export default function Composer({
  placeholder,
  onSend,
}: {
  placeholder: string;
  onSend: (text: string) => Promise<unknown>;
}) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);

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
        <input
          placeholder={placeholder}
          aria-label={placeholder}
          maxLength={500}
          enterKeyHint="send"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <button type="submit" className="primary" aria-label="Send" disabled={!text.trim() || busy}>
          ↑
        </button>
      </div>
    </form>
  );
}
