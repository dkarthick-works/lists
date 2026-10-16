package work.deeka.lists

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import org.dmfs.rfc5545.DateTime
import org.dmfs.rfc5545.recur.RecurrenceRule
import org.json.JSONArray
import org.json.JSONObject
import java.time.Instant
import java.time.LocalDateTime
import java.time.ZoneId
import java.time.temporal.ChronoUnit

// One reminder as the web app hands it over (see frontend/src/android.ts).
data class Reminder(
    val id: String,
    val text: String,
    val listText: String,
    // Wall-clock start, "2026-10-17T08:00", and an optional RRULE repeating it.
    val at: LocalDateTime,
    val rule: String?,
    val completed: Boolean,
    // How the web app words it, e.g. "Every week on Mon, Thu at 08:00".
    val summary: String,
    val listId: String = "",
)

// The phone's copy of the reminders, and the single alarm that rings for them.
//
// Only one alarm is ever set: for the soonest time any reminder (or snooze) is
// due. When it rings, every reminder due at that minute is shown, and the alarm
// is set again for the next soonest. Times are wall-clock in the phone's
// current zone, so 08:00 stays 08:00 after travelling.
object Reminders {
    private const val PREFS = "reminders"
    private const val KEY_LIST = "list" // the JSON array from the web app
    private const val KEY_DONE = "done" // ids ticked off here, not yet sent to the server
    private const val KEY_SNOOZE = "snooze" // id -> epoch millis
    private const val KEY_DUE = "due" // ids the armed alarm rings for
    private const val KEY_DUE_AT = "due_at" // the minute it rings, epoch millis
    const val ACTION_FIRE = "work.deeka.lists.REMINDER_FIRE"

    private fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    @Synchronized
    fun all(ctx: Context): List<Reminder> {
        val arr = JSONArray(prefs(ctx).getString(KEY_LIST, "[]"))
        return (0 until arr.length()).mapNotNull { i ->
            val o = arr.getJSONObject(i)
            runCatching {
                Reminder(
                    id = o.getString("id"),
                    text = o.getString("text"),
                    listText = o.optString("list_text"),
                    at = LocalDateTime.parse(o.getString("remind_at")),
                    rule = o.optString("remind_rule").takeIf { !o.isNull("remind_rule") && it.isNotEmpty() },
                    completed = !o.isNull("completed_at"),
                    summary = o.optString("summary"),
                    listId = o.optString("list_id"),
                )
            }.getOrNull()
        }
    }

    // Replaces the reminders with the server's list. A Done tapped here stays
    // pending until that list shows the entry completed (or gone).
    @Synchronized
    fun replace(ctx: Context, json: String) {
        JSONArray(json) // reject anything that is not a JSON array
        prefs(ctx).edit().putString(KEY_LIST, json).apply()
        val byId = all(ctx).associateBy { it.id }
        val stillPending = done(ctx).filter { id -> byId[id]?.completed == false }
        prefs(ctx).edit().putString(KEY_DONE, JSONArray(stillPending).toString()).apply()
        schedule(ctx)
    }

    @Synchronized
    fun done(ctx: Context): List<String> {
        val arr = JSONArray(prefs(ctx).getString(KEY_DONE, "[]"))
        return (0 until arr.length()).map { arr.getString(it) }
    }

    @Synchronized
    fun markDone(ctx: Context, id: String) {
        val ids = (done(ctx) + id).distinct()
        prefs(ctx).edit().putString(KEY_DONE, JSONArray(ids).toString()).apply()
        setSnooze(ctx, id, null)
        schedule(ctx)
    }

    @Synchronized
    fun snooze(ctx: Context, id: String, minutes: Long) {
        setSnooze(ctx, id, System.currentTimeMillis() + minutes * 60_000)
        schedule(ctx)
    }

    private fun snoozes(ctx: Context): JSONObject = JSONObject(prefs(ctx).getString(KEY_SNOOZE, "{}"))

    private fun setSnooze(ctx: Context, id: String, at: Long?) {
        val s = snoozes(ctx)
        if (at == null) s.remove(id) else s.put(id, at)
        prefs(ctx).edit().putString(KEY_SNOOZE, s.toString()).apply()
    }

    // The reminders the alarm that just rang was set for.
    @Synchronized
    fun due(ctx: Context): Pair<List<Reminder>, Long> {
        val ids = JSONArray(prefs(ctx).getString(KEY_DUE, "[]")).let { a -> (0 until a.length()).map { a.getString(it) } }
        val byId = all(ctx).associateBy { it.id }
        return ids.mapNotNull { byId[it] } to prefs(ctx).getLong(KEY_DUE_AT, 0)
    }

    fun find(ctx: Context, ids: List<String>): List<Reminder> {
        val byId = all(ctx).associateBy { it.id }
        return ids.mapNotNull { byId[it] }
    }

    // Sets the one alarm for the soonest reminder or snooze after `after`
    // (millis), or cancels it when nothing is left.
    @Synchronized
    fun schedule(ctx: Context, after: Long = System.currentTimeMillis()) {
        val zone = ZoneId.systemDefault()
        val from = LocalDateTime.ofInstant(Instant.ofEpochMilli(after), zone)
        val skip = done(ctx).toSet()
        val snoozed = snoozes(ctx)

        // Snoozes that have rung are spent.
        snoozed.keys().asSequence().toList().filter { snoozed.getLong(it) <= after }.forEach { snoozed.remove(it) }
        prefs(ctx).edit().putString(KEY_SNOOZE, snoozed.toString()).apply()

        val times = mutableMapOf<String, Long>()
        for (r in all(ctx)) {
            if (r.completed || r.id in skip) continue
            val next = nextAfter(r, from)?.atZone(zone)?.toInstant()?.toEpochMilli()
            val snooze = if (snoozed.has(r.id)) snoozed.getLong(r.id) else null
            listOfNotNull(next, snooze).minOrNull()?.let { times[r.id] = it }
        }

        val am = ctx.getSystemService(AlarmManager::class.java)
        val fire = PendingIntent.getBroadcast(
            ctx, 0, Intent(ctx, ReminderReceiver::class.java).setAction(ACTION_FIRE),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val soonest = times.values.minOrNull()
        if (soonest == null) {
            am.cancel(fire)
            prefs(ctx).edit().remove(KEY_DUE).remove(KEY_DUE_AT).apply()
            return
        }
        // Everything due in the same minute rings together.
        val minute = soonest / 60_000
        val ids = times.filterValues { it / 60_000 == minute }.keys
        prefs(ctx).edit().putString(KEY_DUE, JSONArray(ids).toString()).putLong(KEY_DUE_AT, soonest).apply()

        if (Build.VERSION.SDK_INT < 31 || am.canScheduleExactAlarms()) {
            // An alarm clock is exact even in Doze. Tapping its status-bar icon opens the app.
            val show = PendingIntent.getActivity(ctx, 0, Intent(ctx, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE)
            am.setAlarmClock(AlarmManager.AlarmClockInfo(soonest, show), fire)
        } else {
            // Without "Alarms & reminders" access Android may ring this some minutes late.
            am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, soonest, fire)
        }
    }

    // The first time r rings strictly after `from`, or null when it is over.
    fun nextAfter(r: Reminder, from: LocalDateTime): LocalDateTime? {
        if (r.rule == null) return r.at.takeIf { it > from }
        val iterator = runCatching {
            RecurrenceRule(r.rule).iterator(DateTime(r.at.year, r.at.monthValue - 1, r.at.dayOfMonth, r.at.hour, r.at.minute, 0))
        }.getOrNull() ?: return null
        // Skip to near `from` instead of walking years of daily occurrences.
        if (from > r.at) iterator.fastForward(dateTime(from.minusDays(1)))
        repeat(10_000) {
            if (!iterator.hasNext()) return null
            val d = iterator.nextDateTime()
            val t = LocalDateTime.of(d.year, d.month + 1, d.dayOfMonth, d.hours, d.minutes).truncatedTo(ChronoUnit.MINUTES)
            if (t > from) return t
        }
        return null
    }

    private fun dateTime(t: LocalDateTime) = DateTime(t.year, t.monthValue - 1, t.dayOfMonth, t.hour, t.minute, 0)
}
