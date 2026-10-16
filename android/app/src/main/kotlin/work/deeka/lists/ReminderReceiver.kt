package work.deeka.lists

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.provider.Settings

// Rings for due reminders, and re-arms the alarm whenever Android may have
// dropped it or moved the clock: reboot, app update, clock or zone change,
// and "Alarms & reminders" access being granted.
class ReminderReceiver : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        if (intent.action != Reminders.ACTION_FIRE) {
            Reminders.schedule(ctx)
            return
        }
        // The ids come from storage, not the intent, so another app sending this
        // action can at most ring what is genuinely due.
        val (due, at) = Reminders.due(ctx)
        Reminders.schedule(ctx, after = maxOf(at, System.currentTimeMillis()))
        if (due.isEmpty()) return
        show(ctx, due.map { it.id }, at)
    }

    companion object {
        const val CHANNEL = "reminders"
        // For when the screen opens by itself: a heads-up banner would only cover it.
        const val QUIET_CHANNEL = "reminders_open"
        const val NOTIFICATION_ID = 1

        fun show(ctx: Context, ids: List<String>, at: Long) {
            val screen = ReminderActivity.intent(ctx, ids, at)
            val open = PendingIntent.getActivity(
                ctx, 1, screen, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
            )
            val nm = ctx.getSystemService(NotificationManager::class.java)
            // Both silent: the reminder screen plays the sound itself, so it can
            // stay quiet under Do Not Disturb and stop the moment it is answered.
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL, "Reminders", NotificationManager.IMPORTANCE_HIGH).apply {
                    setSound(null, null)
                    enableVibration(false)
                },
            )
            nm.createNotificationChannel(
                NotificationChannel(QUIET_CHANNEL, "Open reminders", NotificationManager.IMPORTANCE_LOW),
            )
            // With "Display over other apps" Android lets us open the screen
            // straight over any app, locked or not; the notification is then just
            // a way back to it. Without it, a full-screen notification takes over
            // a locked phone, and an unlocked one only gets a banner.
            val direct = Settings.canDrawOverlays(ctx)
            val first = Reminders.find(ctx, ids).firstOrNull()?.text ?: "Reminder"
            val more = if (ids.size > 1) " (+${ids.size - 1} more)" else ""
            val notification = Notification.Builder(ctx, if (direct) QUIET_CHANNEL else CHANNEL)
                .setSmallIcon(android.R.drawable.ic_lock_idle_alarm)
                .setContentTitle(first + more)
                .setContentText("Tap to open the reminder")
                .setCategory(Notification.CATEGORY_ALARM)
                .setContentIntent(open)
                .setOngoing(true)
            if (!direct) notification.setFullScreenIntent(open, true)
            nm.notify(NOTIFICATION_ID, notification.build())
            if (direct) ctx.startActivity(screen)
        }
    }
}
