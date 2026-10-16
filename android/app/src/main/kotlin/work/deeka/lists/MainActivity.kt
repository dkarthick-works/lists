package work.deeka.lists

import android.Manifest
import android.annotation.SuppressLint
import android.app.Activity
import android.app.AlarmManager
import android.app.NotificationManager
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.view.WindowInsets
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import android.window.OnBackInvokedDispatcher

// A thin shell: the whole app is the Lists web UI running in a WebView.
class MainActivity : Activity() {
    private lateinit var web: WebView

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        web = WebView(this)
        val root = FrameLayout(this).apply { addView(web) }
        setContentView(root)

        // The window is edge-to-edge on Android 15+. Keep the page clear of the
        // system bars, and of the keyboard so the bottom input bar stays visible.
        // The padding goes on a wrapper because a WebView ignores its own.
        root.setOnApplyWindowInsetsListener { v, insets ->
            val i = insets.getInsets(WindowInsets.Type.systemBars() or WindowInsets.Type.ime())
            v.setPadding(i.left, i.top, i.right, i.bottom)
            WindowInsets.CONSUMED
        }

        web.settings.javaScriptEnabled = true
        web.settings.domStorageEnabled = true
        val appHost = Uri.parse(BuildConfig.BASE_URL).host
        val bridge = ReminderBridge(applicationContext, appHost) { askForReminderAccess() }
        web.addJavascriptInterface(bridge, "Android")
        web.webViewClient = object : WebViewClient() {
            override fun onPageStarted(view: WebView, url: String, favicon: Bitmap?) {
                bridge.pageHost = Uri.parse(url).host
            }

            // The app stays in the WebView; anything else (e.g. links to the
            // sign-in service) opens in the browser.
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                if (request.url.host == appHost) return false
                startActivity(Intent(Intent.ACTION_VIEW, request.url))
                return true
            }
        }

        // Back steps through the web app's history before leaving the app.
        if (android.os.Build.VERSION.SDK_INT >= 33) {
            onBackInvokedDispatcher.registerOnBackInvokedCallback(OnBackInvokedDispatcher.PRIORITY_DEFAULT) { goBack() }
        }

        if (savedInstanceState == null) web.loadUrl(BuildConfig.BASE_URL) else web.restoreState(savedInstanceState)
    }

    // The web app listens for this to send the phone any reminder changes made
    // elsewhere, and to pick up Done taps from the reminder screen.
    override fun onResume() {
        super.onResume()
        web.evaluateJavascript("dispatchEvent(new Event('android-resume'))", null)
        if (Reminders.all(this).isNotEmpty()) askForReminderAccess()
    }

    // Opens, once each, the screens that grant what reminders need: notifications,
    // exact alarms, full-screen reminders and drawing over other apps. Coming
    // back here (onResume) moves on to the next one.
    private fun askForReminderAccess() {
        val asked = getSharedPreferences("asked", MODE_PRIVATE)
        val pkg = Uri.parse("package:$packageName")
        val steps: List<Triple<String, Boolean, () -> Unit>> = listOf(
            Triple(
                "notifications",
                Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED,
            ) { requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 0) },
            Triple("exact_alarms", Build.VERSION.SDK_INT >= 31 && !getSystemService(AlarmManager::class.java).canScheduleExactAlarms()) {
                startActivity(Intent(Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM, pkg))
            },
            Triple("full_screen", Build.VERSION.SDK_INT >= 34 && !getSystemService(NotificationManager::class.java).canUseFullScreenIntent()) {
                startActivity(Intent(Settings.ACTION_MANAGE_APP_USE_FULL_SCREEN_INTENT, pkg))
            },
            Triple("overlay", !Settings.canDrawOverlays(this)) {
                startActivity(Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION, pkg))
            },
        )
        val next = steps.firstOrNull { (key, missing) -> missing && !asked.getBoolean(key, false) } ?: return
        asked.edit().putBoolean(next.first, true).apply()
        next.third()
    }

    private fun goBack() {
        if (web.canGoBack()) web.goBack() else finish()
    }

    @Deprecated("Used on Android 12 and below; newer versions use the callback above.")
    override fun onBackPressed() = goBack()

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        web.saveState(outState)
    }
}
