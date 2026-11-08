package net.veil.android

import android.annotation.SuppressLint
import android.graphics.Bitmap
import android.net.Uri
import android.net.http.SslCertificate
import android.net.http.SslError
import android.util.Base64
import android.view.View
import android.view.ViewGroup
import android.webkit.SslErrorHandler
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import android.widget.ProgressBar
import java.io.ByteArrayInputStream
import java.security.MessageDigest
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate

/**
 * Настройки сервера — веб-админка сервера внутри приложения. Запросы идут по туннелю (приложение не исключено из VPN),
 * а сервер узнаёт администратора по ключу туннеля, поэтому пароль админки не нужен. Сертификат сервера самоподписанный:
 * он принимается, только если его отпечаток SPKI совпал с закреплённым в профиле.
 */
class AdminWebPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.ac_server_settings)
    override val scroll = false
    private var root: FrameLayout? = null
    private var web: WebView? = null

    @SuppressLint("SetJavaScriptEnabled")
    override fun build(): View {
        // страница перестраивается при смене состояния VPN и т. п.: панель не перезагружаем, а переносим как есть
        root?.let { (it.parent as? ViewGroup)?.removeView(it); return it }
        val info = Access.info(a)
        val url = info?.optString("admin_url").orEmpty()
        val pin = info?.optString("cert_pin").orEmpty()
        val box = FrameLayout(a).apply { setBackgroundColor(T.p.surface) }
        if (url.isEmpty()) {
            box.addView(a.text(a.getString(R.string.ac_need_admin), 14f, 500, T.p.muted).also { it.setPadding(dp(20), dp(20), dp(20), dp(20)) })
            root = box
            return box
        }
        val host = Uri.parse(url).host
        val bar = ProgressBar(a, null, android.R.attr.progressBarStyleHorizontal).apply { isIndeterminate = true }
        val w = WebView(a).apply {
            setBackgroundColor(T.p.surface)
            settings.javaScriptEnabled = true      // панель — одностраничное приложение
            settings.domStorageEnabled = true
            settings.allowFileAccess = false
            settings.allowContentAccess = false
            webViewClient = object : WebViewClient() {
                override fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: SslError) {
                    if (Uri.parse(error.url).host == host && pinMatches(error.certificate, pin)) handler.proceed() else handler.cancel()
                }
                // только адрес админки; всё прочее (ссылки наружу) — не открываем во встроенном окне
                override fun shouldOverrideUrlLoading(view: WebView, req: WebResourceRequest) = req.url.host != host
                override fun onPageStarted(view: WebView, u: String?, favicon: Bitmap?) { bar.visibility = View.VISIBLE }
                override fun onPageFinished(view: WebView, u: String?) { bar.visibility = View.GONE }
                override fun onReceivedError(view: WebView, req: WebResourceRequest, err: android.webkit.WebResourceError) {
                    if (req.isForMainFrame) {
                        bar.visibility = View.GONE
                        a.toast(a.getString(R.string.ac_server_settings_error))
                    }
                }
            }
        }
        box.addView(w, FrameLayout.LayoutParams(MATCH, MATCH))
        box.addView(bar, FrameLayout.LayoutParams(MATCH, dp(4)))
        w.loadUrl(url)
        web = w; root = box
        return box
    }

    /** «Назад» сначала листает историю панели, затем закрывает страницу. */
    override fun onBack(): Boolean {
        val w = web ?: return false
        if (w.canGoBack()) { w.goBack(); return true }
        return false
    }

    private fun pinMatches(c: SslCertificate?, pin: String): Boolean {
        if (pin.isEmpty()) return true   // профиль без отпечатка: канал и так внутри туннеля, сервер узнаёт клиента по ключу
        return try {
            val der = SslCertificate.saveState(c ?: return false).getByteArray("x509-certificate") ?: return false
            val x = CertificateFactory.getInstance("X.509").generateCertificate(ByteArrayInputStream(der)) as X509Certificate
            val sum = MessageDigest.getInstance("SHA-256").digest(x.publicKey.encoded)
            Base64.encodeToString(sum, Base64.NO_WRAP) == pin
        } catch (_: Exception) { false }
    }
}
