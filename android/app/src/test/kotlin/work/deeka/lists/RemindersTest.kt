package work.deeka.lists

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.time.LocalDateTime

// The phone must ring on the same days the web picker previews
// (frontend/reminder.check.ts covers the same cases there).
class RemindersTest {
    private fun r(at: String, rule: String?) = Reminder("id", "text", "list", LocalDateTime.parse(at), rule, false, "")

    private fun next(r: Reminder, from: String, n: Int): List<String> {
        var t = LocalDateTime.parse(from)
        return (1..n).mapNotNull { Reminders.nextAfter(r, t)?.also { t = it }?.toString() }
    }

    @Test
    fun once() {
        assertEquals(listOf("2026-10-17T08:00"), next(r("2026-10-17T08:00", null), "2026-10-16T12:00", 3))
        assertNull(Reminders.nextAfter(r("2026-10-16T09:00", null), LocalDateTime.parse("2026-10-16T12:00")))
    }

    @Test
    fun weekly() {
        val mt = r("2026-10-19T08:00", "FREQ=WEEKLY;BYDAY=MO,TH")
        assertEquals(listOf("2026-10-19T08:00", "2026-10-22T08:00", "2026-10-26T08:00"), next(mt, "2026-10-16T12:00", 3))
        val fortnight = r("2026-10-17T08:00", "FREQ=WEEKLY;INTERVAL=2")
        assertEquals(listOf("2026-10-17T08:00", "2026-10-31T08:00", "2026-11-14T08:00"), next(fortnight, "2026-10-16T12:00", 3))
    }

    @Test
    fun endsAndMonths() {
        assertEquals(listOf("2026-10-17T08:00", "2026-10-18T08:00"), next(r("2026-10-17T08:00", "FREQ=DAILY;COUNT=2"), "2026-10-16T12:00", 5))
        assertEquals(listOf("2026-10-31T08:00", "2026-12-31T08:00", "2027-01-31T08:00"), next(r("2026-10-31T08:00", "FREQ=MONTHLY;BYMONTHDAY=31"), "2026-10-16T12:00", 3))
        assertEquals(listOf("2026-10-30T08:00", "2026-11-27T08:00"), next(r("2026-10-30T08:00", "FREQ=MONTHLY;BYDAY=-1FR;UNTIL=20261231T235959"), "2026-10-16T12:00", 5).take(2))
        assertEquals(3, next(r("2026-10-30T08:00", "FREQ=MONTHLY;BYDAY=-1FR;UNTIL=20261231T235959"), "2026-10-16T12:00", 5).size)
    }

    @Test
    fun longRunningSeriesSkipsAhead() {
        assertEquals(listOf("2030-01-01T07:30"), next(r("2020-01-01T07:30", "FREQ=DAILY"), "2029-12-31T23:00", 1))
    }
}
