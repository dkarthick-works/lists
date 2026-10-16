package work.deeka.lists

import android.app.Activity
import android.app.NotificationManager
import android.content.Context
import android.content.Intent
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.media.AudioAttributes
import android.media.Ringtone
import android.media.RingtoneManager
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.WindowInsets
import android.widget.LinearLayout
import android.widget.TextView
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

// The full-screen reminder, shown over the lock screen or any app. Reminders
// due together are answered one after another. It rings (and vibrates) until
// answered or for ten minutes, and stays silent under Do Not Disturb.
class ReminderActivity : Activity() {
    private val queue = mutableListOf<Reminder>()
    private var answered = 0
    private var at = 0L
    private var lastAnswer = ""
    private var lastDoneTap = 0L
    private var ringtone: Ringtone? = null
    private var vibrator: Vibrator? = null
    private val handler = Handler(Looper.getMainLooper())
    private lateinit var root: LinearLayout

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setShowWhenLocked(true)
        setTurnScreenOn(true)
        root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setBackgroundColor(Color.WHITE)
        }
        // Edge-to-edge on Android 15+: keep clear of the system bars.
        root.setOnApplyWindowInsetsListener { v, insets ->
            val i = insets.getInsets(WindowInsets.Type.systemBars())
            v.setPadding(i.left + dp(20), i.top + dp(16), i.right + dp(20), i.bottom + dp(20))
            WindowInsets.CONSUMED
        }
        setContentView(root)
        add(intent)
        if (queue.isEmpty()) return finish()
        render()
        startAlert()
        quietNotification()
    }

    // The screen rings now, so the notification stops ringing and stays only
    // as a way back here.
    private fun quietNotification() = ReminderReceiver.notify(this, queue.map { it.id }, at, ringing = false)

    // Rang again while open (a second alarm, or the notification tapped).
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        add(intent)
        render()
        quietNotification()
    }

    private fun add(intent: Intent) {
        val ids = intent.getStringArrayListExtra(EXTRA_IDS) ?: return
        at = intent.getLongExtra(EXTRA_AT, at)
        queue += Reminders.find(this, ids).filter { new -> queue.none { it.id == new.id } }
    }

    private fun answer(what: String, action: (Reminder) -> Unit) {
        val r = queue.removeAt(0)
        action(r)
        answered++
        lastAnswer = "${r.text}: $what"
        if (queue.isEmpty()) {
            ReminderReceiver.cancel(this)
            finish()
        } else {
            render()
        }
    }

    private fun render() {
        root.removeAllViews()
        val r = queue.first()
        val total = answered + queue.size

        root.addView(LinearLayout(this).apply {
            addView(label(if (total > 1) "${answered + 1} of $total due" else "Reminder", 12f).caps(), LinearLayout.LayoutParams(0, -2, 1f))
            addView(label(if (silent()) "Silent · Do Not Disturb" else "♪ Ringing", 12f).caps())
        })
        if (lastAnswer.isNotEmpty()) {
            root.addView(label(lastAnswer, 14f).boxed(), match().apply { topMargin = dp(12) })
        }
        root.addView(View(this), LinearLayout.LayoutParams(-1, 0, 1f))
        val time = DateTimeFormatter.ofPattern("HH:mm").format(Instant.ofEpochMilli(at).atZone(ZoneId.systemDefault()))
        root.addView(label(listOf(time, r.listText).filter { it.isNotEmpty() }.joinToString(" · "), 13f))
        root.addView(label(r.text, 30f).apply { setTypeface(Typeface.MONOSPACE, Typeface.BOLD) }, match().apply { topMargin = dp(4) })
        if (r.summary.isNotEmpty()) root.addView(label(r.summary, 13f).apply { alpha = 0.55f }, match().apply { topMargin = dp(6) })
        root.addView(View(this), LinearLayout.LayoutParams(-1, 0, 1f))

        // Done takes two taps within 300ms, like the app's delete buttons, so a
        // stray touch in a pocket cannot tick an entry off.
        val done = cell("✓ Done\ndouble-tap", primary = true)
        done.setOnClickListener {
            val now = SystemClock.uptimeMillis()
            if (now - lastDoneTap < DOUBLE_TAP_MS) {
                lastDoneTap = 0
                answer("done") { Reminders.markDone(this, it.id) }
                return@setOnClickListener
            }
            lastDoneTap = now
            done.text = "✓ Done\ntap again"
            handler.postDelayed({ if (SystemClock.uptimeMillis() - lastDoneTap >= DOUBLE_TAP_MS) done.text = "✓ Done\ndouble-tap" }, DOUBLE_TAP_MS + 20)
        }
        root.addView(done, match())
        root.addView(row(snooze(10), snooze(30)))
        root.addView(row(snooze(60), cell("Dismiss").apply {
            // The next time is already scheduled; nothing else to do.
            setOnClickListener { answer("dismissed") {} }
        }))
    }

    private fun snooze(minutes: Long) =
        cell("Snooze\n" + if (minutes < 60) "$minutes min" else "1 hour").apply {
            setOnClickListener { answer("snoozed") { Reminders.snooze(this@ReminderActivity, it.id, minutes) } }
        }

    private fun silent() =
        getSystemService(NotificationManager::class.java).currentInterruptionFilter != NotificationManager.INTERRUPTION_FILTER_ALL

    private fun startAlert() {
        if (silent()) return
        val uri = RingtoneManager.getActualDefaultRingtoneUri(this, RingtoneManager.TYPE_ALARM)
            ?: RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION)
        ringtone = RingtoneManager.getRingtone(this, uri)?.apply {
            audioAttributes = AudioAttributes.Builder()
                .setUsage(AudioAttributes.USAGE_ALARM)
                .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
                .build()
            isLooping = true
            play()
        }
        val v = if (Build.VERSION.SDK_INT >= 31) getSystemService(VibratorManager::class.java).defaultVibrator else getSystemService(Vibrator::class.java)
        vibrator = v.apply {
            vibrate(VibrationEffect.createWaveform(longArrayOf(0, 600, 900), 0))
        }
        // ponytail: fixed 10-minute cap, like a clock app's default; make it a setting if it ever needs changing.
        handler.postDelayed(::stopAlert, 10 * 60_000L)
    }

    private fun stopAlert() {
        ringtone?.stop()
        ringtone = null
        vibrator?.cancel()
        vibrator = null
    }

    override fun onDestroy() {
        stopAlert()
        handler.removeCallbacksAndMessages(null)
        super.onDestroy()
    }

    // Views, in the web app's style: black on white, monospace, square 1dp borders.

    private fun dp(v: Int) = TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, v.toFloat(), resources.displayMetrics).toInt()

    private fun match() = LinearLayout.LayoutParams(-1, -2)

    private fun border(fill: Int) = GradientDrawable().apply {
        setColor(fill)
        setStroke(dp(1).coerceAtLeast(1), Color.BLACK)
    }

    private fun label(text: String, sp: Float) = TextView(this).apply {
        this.text = text
        setTextColor(Color.BLACK)
        setTextSize(TypedValue.COMPLEX_UNIT_SP, sp)
        typeface = Typeface.MONOSPACE
    }

    private fun TextView.caps() = apply {
        isAllCaps = true
        letterSpacing = 0.08f
    }

    private fun TextView.boxed() = apply {
        background = border(Color.WHITE)
        setPadding(dp(12), dp(8), dp(12), dp(8))
    }

    private fun cell(text: String, primary: Boolean = false) = label(text, 16f).apply {
        gravity = Gravity.CENTER
        minHeight = dp(88)
        if (primary) setTypeface(Typeface.MONOSPACE, Typeface.BOLD)
        setTextColor(if (primary) Color.WHITE else Color.BLACK)
        background = border(if (primary) Color.BLACK else Color.WHITE)
        isClickable = true
        isFocusable = true
    }

    // Two cells side by side; the -1dp margins collapse their shared borders.
    private fun row(a: View, b: View) = LinearLayout(this).apply {
        // Otherwise a one-line cell is shifted to line up with a two-line one's text.
        isBaselineAligned = false
        addView(a, LinearLayout.LayoutParams(0, -2, 1f))
        addView(b, LinearLayout.LayoutParams(0, -2, 1f).apply { leftMargin = -dp(1) })
        layoutParams = match().apply { topMargin = -dp(1) }
    }

    companion object {
        private const val EXTRA_IDS = "ids"
        private const val EXTRA_AT = "at"
        private const val DOUBLE_TAP_MS = 300L

        fun intent(ctx: Context, ids: List<String>, at: Long): Intent =
            Intent(ctx, ReminderActivity::class.java)
                .putStringArrayListExtra(EXTRA_IDS, ArrayList(ids))
                .putExtra(EXTRA_AT, at)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    }
}
