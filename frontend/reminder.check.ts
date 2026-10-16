// Self-check for reminder.ts: node reminder.check.ts
import assert from "node:assert/strict";
import { aligned, fromStored, newRule, quickDays, summary, toRRule, upcoming, type Rule } from "./src/reminder.ts";

const now = new Date(2026, 9, 16, 12, 0); // Fri 16 Oct 2026, 12:00 local
const rule = (patch: Partial<Rule>): Rule => ({ ...newRule("2026-10-17"), ...patch });

// Round trip through what the server stores.
for (const r of [
  rule({}),
  rule({ freq: "DAILY", interval: 3 }),
  rule({ freq: "WEEKLY", byday: ["TH", "MO"], end: { type: "count", count: 4, until: "" } }),
  rule({ date: "2026-10-30", freq: "MONTHLY", monthMode: "last" }),
  rule({
    date: "2026-11-10",
    freq: "MONTHLY",
    monthMode: "nth",
    end: { type: "until", until: "2027-06-30", count: 5 },
  }),
]) {
  const back = fromStored(`${r.date}T${r.time}`, toRRule(r));
  assert.equal(toRRule(back), toRRule(r));
}

assert.equal(toRRule(rule({ freq: "WEEKLY", byday: ["TH", "MO"] })), "FREQ=WEEKLY;BYDAY=MO,TH");
assert.equal(toRRule(rule({ date: "2026-10-30", freq: "MONTHLY", monthMode: "last" })), "FREQ=MONTHLY;BYDAY=-1FR");

// A Mon/Thu rule started on Saturday 17th begins on Monday 19th.
const mt = aligned(rule({ freq: "WEEKLY", byday: ["MO", "TH"] }));
assert.equal(mt.date, "2026-10-19");
assert.deepEqual(upcoming(mt, 3, now), ["2026-10-19", "2026-10-22", "2026-10-26"]);

// Every other week, COUNT stops it, day 31 skips short months.
assert.deepEqual(upcoming(rule({ freq: "WEEKLY", interval: 2 }), 3, now), ["2026-10-17", "2026-10-31", "2026-11-14"]);
assert.deepEqual(upcoming(rule({ freq: "DAILY", end: { type: "count", count: 2, until: "" } }), 5, now), [
  "2026-10-17",
  "2026-10-18",
]);
assert.deepEqual(upcoming(rule({ date: "2026-10-31", freq: "MONTHLY" }), 3, now), [
  "2026-10-31",
  "2026-12-31",
  "2027-01-31",
]);

// A one-off earlier today has nothing left.
assert.deepEqual(upcoming(newRule("2026-10-16", "09:00"), 5, now), []);

assert.equal(summary(rule({ freq: "WEEKLY", byday: ["MO", "TU", "WE", "TH", "FR"] }), now), "Every weekday at 08:00");
assert.equal(summary(rule({}), now), "Tomorrow at 08:00");
// Quick picks never repeat a day: Fri 16 Oct gives Sun instead of a second Saturday.
const labels = (t: string) =>
  quickDays(t)
    .map(([l]) => l)
    .join(" ");
assert.equal(labels("2026-10-16"), "Today Tomorrow Sun Mon"); // Fri
assert.equal(labels("2026-10-18"), "Today Tomorrow Tue Sat"); // Sun
assert.equal(labels("2026-10-15"), "Today Tomorrow Sat Mon"); // Thu
for (let i = 0; i < 7; i++) {
  const days = quickDays(`2026-10-${12 + i}`).map(([, d]) => d);
  assert.equal(new Set(days).size, 4);
}
console.log("reminder.ts ok");
