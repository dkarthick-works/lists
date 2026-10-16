import { lists } from "./api";

// The Android shell adds window.Android (see ReminderBridge.kt). In a browser it
// is missing and reminders are not offered: a web page cannot ring an alarm.
interface Bridge {
  // Every reminder, as GET /api/reminders returns them; the phone reschedules.
  setReminders(json: string): void;
  // Ids of entries ticked off from the reminder screen, not yet sent here.
  pendingDone(): string;
}

const bridge = () => (window as { Android?: Bridge }).Android;
export const isAndroid = () => bridge() !== undefined;

let queue: Promise<void> = Promise.resolve();

// Sends Done taps from the reminder screen to the server, then hands the phone
// the full reminder list. The phone drops a pending Done once the list shows
// that entry completed, so a failed send is retried on the next sync. Syncs
// run one after another, so the last one always reflects the last change.
export function syncReminders(): Promise<void> {
  const b = bridge();
  if (!b) return Promise.resolve();
  queue = queue
    .then(async () => {
      const done: string[] = JSON.parse(b.pendingDone() || "[]");
      // A deleted entry (404) has nothing left to complete.
      await Promise.all(done.map((id) => lists.setCompleted(id, true).catch(() => {})));
      b.setReminders(JSON.stringify(await lists.reminders()));
    })
    .catch(() => {}); // offline: the next sync catches up
  return queue;
}
