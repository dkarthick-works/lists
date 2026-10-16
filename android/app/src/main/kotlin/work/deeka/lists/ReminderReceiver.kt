package work.deeka.lists

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.graphics.PixelFormat
import android.media.AudioAttributes
import android.media.RingtoneManager
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.view.View
import android.view.WindowManager

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
        val ids = due.map { it.id }
        notify(ctx, ids, at, ringing = true)
        if (Settings.canDrawOverlays(ctx)) openOverOtherApps(ctx, ReminderActivity.intent(ctx, ids, at), goAsync())
    }

    companion object {
        // Rings until the reminder screen opens: a full-screen notification takes
        // over a locked phone, and an unlocked one at least gets a ringing banner.
        private const val RINGING = "reminders_ringing"
        // Once the screen is open the notification is only a way back to it.
        private const val QUIET = "reminders_open"
        private const val NOTIFICATION_ID = 1

        fun notify(ctx: Context, ids: List<String>, at: Long, ringing: Boolean) {
            val nm = ctx.getSystemService(NotificationManager::class.java)
            nm.deleteNotificationChannel("reminders") // silent; replaced by RINGING
            nm.createNotificationChannel(
                NotificationChannel(RINGING, "Reminders", NotificationManager.IMPORTANCE_HIGH).apply {
                    // Notification usage, not alarm, so Do Not Disturb silences it.
                    setSound(
                        RingtoneManager.getDefaultUri(RingtoneManager.TYPE_ALARM),
                        AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_NOTIFICATION_EVENT).build(),
                    )
                    enableVibration(true)
                },
            )
            nm.createNotificationChannel(NotificationChannel(QUIET, "Open reminders", NotificationManager.IMPORTANCE_LOW))

            val open = PendingIntent.getActivity(
                ctx, 1, ReminderActivity.intent(ctx, ids, at), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
            )
            val first = Reminders.find(ctx, ids).firstOrNull()?.text ?: "Reminder"
            val more = if (ids.size > 1) " (+${ids.size - 1} more)" else ""
            val builder = Notification.Builder(ctx, if (ringing) RINGING else QUIET)
                .setSmallIcon(android.R.drawable.ic_lock_idle_alarm)
                .setContentTitle(first + more)
                .setContentText("Tap to open the reminder")
                .setCategory(Notification.CATEGORY_ALARM)
                .setContentIntent(open)
                .setOngoing(true)
            if (ringing) builder.setFullScreenIntent(open, true)
            val n = builder.build()
            if (ringing) n.flags = n.flags or Notification.FLAG_INSISTENT // repeat the sound until answered
            nm.notify(NOTIFICATION_ID, n)
        }

        fun cancel(ctx: Context) = ctx.getSystemService(NotificationManager::class.java).cancel(NOTIFICATION_ID)

        // Since Android 15, "Display over other apps" only lets an app open a
        // screen from the background while one of its overlay windows is
        // visible. So show an invisible 1px overlay, open the reminder over
        // whatever app is in use, then take the overlay away.
        private fun openOverOtherApps(ctx: Context, screen: Intent, pending: PendingResult) {
            val wm = ctx.getSystemService(WindowManager::class.java)
            val dot = View(ctx)
            val params = WindowManager.LayoutParams(
                1, 1,
                WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY,
                WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE,
                PixelFormat.TRANSLUCENT,
            )
            if (runCatching { wm.addView(dot, params) }.isFailure) return pending.finish()
            val main = Handler(Looper.getMainLooper())
            // ponytail: fixed waits for the overlay to count as visible; switch to a draw listener if a phone needs longer.
            main.postDelayed({
                runCatching { ctx.startActivity(screen) }
                main.postDelayed({
                    runCatching { wm.removeView(dot) }
                    pending.finish()
                }, 1000)
            }, 300)
        }
    }
}
