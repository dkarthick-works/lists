import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { syncReminders } from "./android";
import { lists, type ListDetail } from "./api";
import {
  addDays,
  aligned,
  dayLabel,
  DOW,
  DOW_NAME,
  fromStored,
  newRule,
  nextWhere,
  nthInfo,
  ORD,
  summary,
  today,
  toRRule,
  UNIT,
  upcoming,
  weekday,
  type EndType,
  type Freq,
  type MonthMode,
  type Rule,
} from "./reminder";

const WEEK = ["MO", "TU", "WE", "TH", "FR", "SA", "SU"];

// A row of buttons that act like radio buttons.
function Segments<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: [T, string][];
  onChange: (v: T) => void;
}) {
  return (
    <div className="seg" role="radiogroup" aria-label={label}>
      {options.map(([v, text]) => (
        <button
          key={v}
          type="button"
          role="radio"
          aria-checked={v === value}
          className={v === value ? "on" : undefined}
          onClick={() => onChange(v)}
        >
          {text}
        </button>
      ))}
    </div>
  );
}

const count = (raw: string) => Math.min(999, Math.max(1, Number(raw) || 1));

// Full-page form for an entry's reminder: when it first fires and how it repeats.
export default function ReminderPage() {
  const { id = "", entryId = "" } = useParams();
  const navigate = useNavigate();
  const [data, setData] = useState<ListDetail | null>(null);
  const [rule, setRule] = useState<Rule | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    lists
      .get(id)
      .then((d) => {
        const e = d.entries.find((x) => x.id === entryId);
        if (!e) return setError("Entry not found.");
        setData(d);
        setRule(e.remind_at ? fromStored(e.remind_at, e.remind_rule) : newRule(addDays(today(), 1)));
      })
      .catch((e) => setError(e.message));
  }, [id, entryId]);

  const entry = data?.entries.find((x) => x.id === entryId);
  if (!data || !entry || !rule) return <p className={error ? undefined : "muted"}>{error || "Loading…"}</p>;

  const set = (patch: Partial<Rule>) => setRule({ ...rule, ...patch });
  const setDate = (date: string) => {
    if (!date) return;
    // "Last Friday" no longer fits a date outside the month's final week.
    const keepLast = rule.monthMode !== "last" || nthInfo(date).last;
    set({ date, monthMode: keepLast ? rule.monthMode : "day" });
  };
  const toggleDay = (d: string) => {
    const on = rule.byday.length ? rule.byday : [DOW[weekday(rule.date)]];
    const next = on.includes(d) ? on.filter((x) => x !== d) : [...on, d];
    set({ byday: next.length ? next : [d] });
  };

  const t = today();
  const info = nthInfo(rule.date);
  const next = upcoming(rule);
  const chips: [string, string][] = [
    ["Today", t],
    ["Tomorrow", addDays(t, 1)],
    ["Sat", nextWhere(addDays(t, 1), (s) => weekday(s) === 6)],
    ["Mon", nextWhere(addDays(t, 1), (s) => weekday(s) === 1)],
  ];
  const monthOptions: [MonthMode, string][] = [
    ["day", `On day ${info.day}`],
    ["nth", `On the ${ORD[info.n]} ${DOW_NAME[info.dow]}`],
  ];
  if (info.last) monthOptions.push(["last", `Last ${DOW_NAME[info.dow]}`]);

  function save(remindAt: string | null, rrule: string | null) {
    setBusy(true);
    lists
      .setReminder(entry!.id, remindAt, rrule)
      .then(() => syncReminders())
      .then(() => navigate(-1))
      .catch((e) => {
        setError(e.message);
        setBusy(false);
      });
  }

  function submit() {
    const r = aligned(rule!);
    save(`${r.date}T${r.time}`, toRRule(r));
  }

  return (
    <>
      <nav className="crumbs" aria-label="Breadcrumb">
        {[...data.ancestors, data.list].map((a, i) => (
          <span key={a.id}>
            {i > 0 && " / "}
            <Link to={`/lists/${a.id}`}>{a.text}</Link>
          </span>
        ))}
      </nav>
      <div>
        <h1>Remind me</h1>
        <p>{entry.text}</p>
      </div>
      {error && <p role="alert">{error}</p>}

      <section className="stack">
        <h2>When</h2>
        <div className="chips">
          {chips.map(([label, date]) => (
            <button
              key={label}
              type="button"
              className={rule.date === date ? "on" : undefined}
              onClick={() => setDate(date)}
            >
              {label}
            </button>
          ))}
        </div>
        <div className="pair">
          <label>
            Date
            <input type="date" value={rule.date} min={t} onChange={(e) => setDate(e.target.value)} />
          </label>
          <label>
            Time
            <input type="time" value={rule.time} onChange={(e) => e.target.value && set({ time: e.target.value })} />
          </label>
        </div>
      </section>

      <section className="stack">
        <h2>Repeat</h2>
        <Segments<Freq | "">
          label="Repeat"
          value={rule.freq ?? ""}
          options={[
            ["", "Never"],
            ["DAILY", "Daily"],
            ["WEEKLY", "Weekly"],
            ["MONTHLY", "Monthly"],
            ["YEARLY", "Yearly"],
          ]}
          onChange={(v) => set({ freq: v || null })}
        />
        {rule.freq && (
          <>
            <label className="inline-row">
              Every
              <input
                type="number"
                inputMode="numeric"
                min={1}
                max={999}
                value={rule.interval}
                onChange={(e) => set({ interval: count(e.target.value) })}
              />
              {UNIT[rule.freq]}
              {rule.interval > 1 ? "s" : ""}
            </label>
            {rule.freq === "WEEKLY" && (
              <div className="days" role="group" aria-label="Days of the week">
                {WEEK.map((d) => {
                  const on = (rule.byday.length ? rule.byday : [DOW[weekday(rule.date)]]).includes(d);
                  const name = DOW_NAME[DOW.indexOf(d)];
                  return (
                    <button
                      key={d}
                      type="button"
                      aria-pressed={on}
                      aria-label={name}
                      className={on ? "on" : undefined}
                      onClick={() => toggleDay(d)}
                    >
                      {name[0]}
                    </button>
                  );
                })}
              </div>
            )}
            {rule.freq === "MONTHLY" && (
              <Segments
                label="Day of the month"
                value={rule.monthMode}
                options={monthOptions}
                onChange={(v) => set({ monthMode: v })}
              />
            )}
            <div className="stack">
              <span className="small">Ends</span>
              <Segments<EndType>
                label="Ends"
                value={rule.end.type}
                options={[
                  ["never", "Never"],
                  ["until", "On date"],
                  ["count", "After"],
                ]}
                onChange={(v) => set({ end: { ...rule.end, type: v } })}
              />
              {rule.end.type === "until" && (
                <input
                  type="date"
                  aria-label="Last day"
                  value={rule.end.until}
                  min={rule.date}
                  onChange={(e) => e.target.value && set({ end: { ...rule.end, until: e.target.value } })}
                />
              )}
              {rule.end.type === "count" && (
                <label className="inline-row">
                  <input
                    type="number"
                    inputMode="numeric"
                    min={1}
                    max={999}
                    value={rule.end.count}
                    onChange={(e) => set({ end: { ...rule.end, count: count(e.target.value) } })}
                  />
                  times
                </label>
              )}
            </div>
          </>
        )}
      </section>

      <section className="stack">
        <h2>Summary</h2>
        <p className="summary">{summary(rule)}</p>
        {next.length ? (
          <ol className="next">
            {next.map((d) => (
              <li key={d}>
                {dayLabel(d)} · {rule.time}
              </li>
            ))}
          </ol>
        ) : (
          <p>That time has already passed.</p>
        )}
      </section>

      {entry.remind_at && (
        <div>
          <button className="link" disabled={busy} onClick={() => save(null, null)}>
            Remove reminder
          </button>
        </div>
      )}

      <div className="form-foot">
        <div className="confirm-actions">
          <button className="primary" disabled={busy || next.length === 0} onClick={submit}>
            Save
          </button>
          <button onClick={() => navigate(-1)}>Cancel</button>
        </div>
      </div>
    </>
  );
}
