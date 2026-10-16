// Reminder rules: a start (wall-clock date and time, no zone) plus an optional
// RFC 5545 RRULE. The picker edits a Rule; the server stores remind_at and
// remind_rule; the Android app schedules from the same two strings.

export const DEFAULT_TIME = "08:00";

export type Freq = "DAILY" | "WEEKLY" | "MONTHLY" | "YEARLY";
export type MonthMode = "day" | "nth" | "last";
export type EndType = "never" | "until" | "count";

export interface Rule {
  date: string; // YYYY-MM-DD
  time: string; // HH:MM
  freq: Freq | null; // null: once
  interval: number;
  byday: string[]; // weekly only: MO, TU, ...; empty means the start's weekday
  monthMode: MonthMode; // monthly: same date, same nth weekday, or last weekday
  end: { type: EndType; until: string; count: number };
}

export const DOW = ["SU", "MO", "TU", "WE", "TH", "FR", "SA"];
export const DOW_NAME = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MON = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
export const ORD = ["", "1st", "2nd", "3rd", "4th", "5th"];
export const UNIT: Record<Freq, string> = { DAILY: "day", WEEKLY: "week", MONTHLY: "month", YEARLY: "year" };
const DAY = 864e5;

// Dates are handled as UTC midnights so that no local zone or DST shift can
// move them; only the wall-clock fields matter.
const pad = (n: number) => String(n).padStart(2, "0");
const toStr = (d: Date) => `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
const parseD = (s: string) => new Date(`${s}T00:00:00Z`);
export const addDays = (s: string, n: number) => toStr(new Date(parseD(s).getTime() + n * DAY));
export const weekday = (s: string) => parseD(s).getUTCDay();

export function today(now = new Date()) {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}
export const clock = (now = new Date()) => `${pad(now.getHours())}:${pad(now.getMinutes())}`;

export function nextWhere(from: string, pred: (s: string) => boolean) {
  for (let i = 0; i < 400; i++) {
    const s = addDays(from, i);
    if (pred(s)) return s;
  }
  return from;
}

// Quick picks under "When": today, tomorrow, and the coming Saturday and
// Monday. A pick that lands on a day already shown moves to the next free day,
// so the row always offers four different days, in date order.
export function quickDays(t: string): [string, string][] {
  const out: [string, string][] = [
    ["Today", t],
    ["Tomorrow", addDays(t, 1)],
  ];
  for (const dow of [6, 1]) {
    let d = nextWhere(addDays(t, 1), (s) => weekday(s) === dow);
    while (out.some(([, x]) => x === d)) d = addDays(d, 1);
    out.push([DOW_NAME[weekday(d)], d]);
  }
  return [...out.slice(0, 2), ...out.slice(2).sort((a, b) => a[1].localeCompare(b[1]))];
}

// Which weekday of its month a date is: n is 1-5, last is true in the final week.
export function nthInfo(s: string) {
  const d = parseD(s);
  const day = d.getUTCDate();
  const daysInMonth = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 0)).getUTCDate();
  return { n: Math.ceil(day / 7), last: day + 7 > daysInMonth, dow: d.getUTCDay(), day };
}

const mondayFirst = (days: string[]) =>
  [...days].sort((a, b) => ((DOW.indexOf(a) + 6) % 7) - ((DOW.indexOf(b) + 6) % 7));

export function newRule(date: string, time = DEFAULT_TIME): Rule {
  return {
    date,
    time,
    freq: null,
    interval: 1,
    byday: [],
    monthMode: "day",
    end: { type: "never", until: addDays(date, 90), count: 5 },
  };
}

export function toRRule(r: Rule): string | null {
  if (!r.freq) return null;
  const p = [`FREQ=${r.freq}`];
  if (r.interval > 1) p.push(`INTERVAL=${r.interval}`);
  if (r.freq === "WEEKLY" && r.byday.length) p.push(`BYDAY=${mondayFirst(r.byday).join(",")}`);
  if (r.freq === "MONTHLY") {
    const i = nthInfo(r.date);
    if (r.monthMode === "nth") p.push(`BYDAY=${i.n}${DOW[i.dow]}`);
    else if (r.monthMode === "last") p.push(`BYDAY=-1${DOW[i.dow]}`);
    else p.push(`BYMONTHDAY=${i.day}`);
  }
  if (r.end.type === "count") p.push(`COUNT=${r.end.count}`);
  if (r.end.type === "until") p.push(`UNTIL=${r.end.until.replace(/-/g, "")}T235959`);
  return p.join(";");
}

// Reads back what toRRule wrote. remindAt is "YYYY-MM-DDTHH:MM".
export function fromStored(remindAt: string, rrule: string | null): Rule {
  const r = newRule(remindAt.slice(0, 10), remindAt.slice(11, 16));
  for (const part of (rrule ?? "").split(";").filter(Boolean)) {
    const [k, v] = part.split("=");
    if (k === "FREQ") r.freq = v as Freq;
    if (k === "INTERVAL") r.interval = Number(v);
    if (k === "COUNT") r.end = { ...r.end, type: "count", count: Number(v) };
    if (k === "UNTIL") r.end = { ...r.end, type: "until", until: `${v.slice(0, 4)}-${v.slice(4, 6)}-${v.slice(6, 8)}` };
    if (k === "BYDAY") {
      if (/^-1/.test(v)) r.monthMode = "last";
      else if (/^\d/.test(v)) r.monthMode = "nth";
      else r.byday = v.split(",");
    }
  }
  return r;
}

// Whether date s is an occurrence of r, ignoring COUNT and UNTIL.
function matches(r: Rule, s: string) {
  const d = parseD(s);
  const st = parseD(r.date);
  const days = Math.round((d.getTime() - st.getTime()) / DAY);
  if (days < 0) return false;
  const months = (d.getUTCFullYear() - st.getUTCFullYear()) * 12 + d.getUTCMonth() - st.getUTCMonth();
  switch (r.freq) {
    case null:
      return days === 0;
    case "DAILY":
      return days % r.interval === 0;
    case "WEEKLY": {
      const weeks = Math.floor((days + ((st.getUTCDay() + 6) % 7)) / 7);
      const by = r.byday.length ? r.byday : [DOW[st.getUTCDay()]];
      return weeks % r.interval === 0 && by.includes(DOW[d.getUTCDay()]);
    }
    case "MONTHLY": {
      if (months % r.interval) return false;
      const a = nthInfo(s);
      const b = nthInfo(r.date);
      if (r.monthMode === "nth") return a.dow === b.dow && a.n === b.n;
      if (r.monthMode === "last") return a.dow === b.dow && a.last;
      return a.day === b.day;
    }
    case "YEARLY":
      return months % (12 * r.interval) === 0 && d.getUTCDate() === st.getUTCDate();
  }
}

// The rule with its start moved to its first real occurrence (e.g. a weekly
// Mon/Thu rule started on a Saturday starts on the Monday). Saved rules are
// always aligned, so every RRULE reader agrees on where the series begins.
export function aligned(r: Rule): Rule {
  return { ...r, date: nextWhere(r.date, (s) => matches(r, s)) };
}

// The next n dates still ahead of now.
export function upcoming(r: Rule, n = 5, now = new Date()): string[] {
  const out: string[] = [];
  const t = today(now);
  const c = clock(now);
  let k = 0;
  for (let i = 0; i < 3700 && out.length < n; i++) {
    const s = addDays(r.date, i);
    if (!matches(r, s)) continue;
    k++;
    if (r.end.type === "count" && k > r.end.count) break;
    if (r.end.type === "until" && s > r.end.until) break;
    if (s > t || (s === t && r.time > c)) out.push(s);
    if (!r.freq) break;
  }
  return out;
}

export function dayLabel(s: string, now = new Date()) {
  const t = today(now);
  if (s === t) return "Today";
  if (s === addDays(t, 1)) return "Tomorrow";
  const d = parseD(s);
  let out = `${DOW_NAME[d.getUTCDay()]} ${d.getUTCDate()} ${MON[d.getUTCMonth()]}`;
  if (d.getUTCFullYear() !== now.getFullYear()) out += ` ${d.getUTCFullYear()}`;
  return out;
}

export function summary(r: Rule, now = new Date()) {
  const at = ` at ${r.time}`;
  if (!r.freq) return `${dayLabel(r.date, now)}${at}`;
  let s = r.interval > 1 ? `Every ${r.interval} ${UNIT[r.freq]}s` : `Every ${UNIT[r.freq]}`;
  if (r.freq === "WEEKLY") {
    const by = mondayFirst(r.byday.length ? r.byday : [DOW[weekday(r.date)]]).join(",");
    if (by === "MO,TU,WE,TH,FR") s = r.interval > 1 ? `${s} on weekdays` : "Every weekday";
    else if (by === "SA,SU" && r.interval === 1) s = "Every weekend";
    else
      s += ` on ${by
        .split(",")
        .map((d) => DOW_NAME[DOW.indexOf(d)])
        .join(", ")}`;
  }
  if (r.freq === "MONTHLY") {
    const i = nthInfo(r.date);
    if (r.monthMode === "nth") s += ` on the ${ORD[i.n]} ${DOW_NAME[i.dow]}`;
    else if (r.monthMode === "last") s += ` on the last ${DOW_NAME[i.dow]}`;
    else s += ` on day ${i.day}`;
  }
  if (r.freq === "YEARLY") {
    const d = parseD(r.date);
    s += ` on ${d.getUTCDate()} ${MON[d.getUTCMonth()]}`;
  }
  s += at;
  if (r.end.type === "count") s += `, ${r.end.count} times`;
  if (r.end.type === "until") s += `, until ${dayLabel(r.end.until, now)}`;
  return s;
}
