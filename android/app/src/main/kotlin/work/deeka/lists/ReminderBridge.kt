package work.deeka.lists

import android.content.Context
import android.webkit.JavascriptInterface
import org.json.JSONArray

// window.Android in the web app (frontend/src/android.ts). Every page loaded in
// the WebView can see it, so each call checks the page is the app itself;
// MainActivity keeps pageHost up to date as the WebView navigates.
class ReminderBridge(
    private val ctx: Context,
    private val appHost: String?,
    // Runs on the main thread once there is at least one reminder.
    private val onReminders: () -> Unit,
) {
    @Volatile
    var pageHost: String? = null

    private fun trusted() = pageHost != null && pageHost == appHost

    @JavascriptInterface
    fun setReminders(json: String) {
        if (!trusted()) return
        Reminders.replace(ctx, json)
        if (Reminders.all(ctx).isNotEmpty()) android.os.Handler(ctx.mainLooper).post(onReminders)
    }

    @JavascriptInterface
    fun pendingDone(): String = if (trusted()) JSONArray(Reminders.done(ctx)).toString() else "[]"
}
