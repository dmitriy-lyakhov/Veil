package net.veil.android

import android.Manifest
import android.app.Activity
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.view.View
import android.widget.Button
import android.widget.EditText
import android.widget.TextView
import android.widget.Toast
import org.json.JSONObject
import veilcore.Veilcore

class MainActivity : Activity() {

    private lateinit var statusView: TextView
    private lateinit var serverView: TextView
    private lateinit var trafficView: TextView
    private lateinit var errorView: TextView
    private lateinit var linkView: EditText
    private lateinit var connectBtn: Button

    private val handler = Handler(Looper.getMainLooper())
    private var pending = false // запрос подключения отправлен, ждём ответ ядра

    private val poll = object : Runnable {
        override fun run() {
            render()
            handler.postDelayed(this, 1000)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        statusView = findViewById(R.id.status)
        serverView = findViewById(R.id.server)
        trafficView = findViewById(R.id.traffic)
        errorView = findViewById(R.id.error)
        linkView = findViewById(R.id.link)
        connectBtn = findViewById(R.id.connect)

        linkView.setText(Store.loadLink(this))
        findViewById<Button>(R.id.paste).setOnClickListener { pasteFromClipboard() }
        connectBtn.setOnClickListener { onConnectClick() }
        handleIntent(intent)

        if (Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 2)
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleIntent(intent)
    }

    private fun handleIntent(i: Intent?) {
        val d = i?.dataString ?: return
        if (d.startsWith("veil://")) {
            linkView.setText(d)
            Store.saveLink(this, d)
        }
    }

    override fun onResume() {
        super.onResume()
        handler.post(poll)
    }

    override fun onPause() {
        handler.removeCallbacks(poll)
        super.onPause()
    }

    private fun pasteFromClipboard() {
        val cm = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        val t = cm.primaryClip?.takeIf { it.itemCount > 0 }?.getItemAt(0)?.coerceToText(this)?.toString()?.trim()
        if (t.isNullOrEmpty()) {
            Toast.makeText(this, "Буфер обмена пуст", Toast.LENGTH_SHORT).show()
        } else {
            linkView.setText(t)
        }
    }

    private fun running(): Boolean = try {
        JSONObject(Veilcore.status()).optString("state") != "stopped"
    } catch (_: Exception) { false }

    private fun onConnectClick() {
        if (running() || pending) {
            pending = false
            startService(Intent(this, VeilVpnService::class.java).setAction(VeilVpnService.ACTION_DISCONNECT))
            return
        }
        val link = linkView.text.toString().trim()
        try {
            Veilcore.parse(link)
        } catch (e: Exception) {
            Toast.makeText(this, e.message ?: "Некорректная ссылка", Toast.LENGTH_LONG).show()
            return
        }
        Store.saveLink(this, link)
        Store.lastError = null
        val prep = VpnService.prepare(this)
        if (prep != null) {
            @Suppress("DEPRECATION")
            startActivityForResult(prep, REQ_VPN)
        } else {
            startVpn()
        }
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        @Suppress("DEPRECATION")
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == REQ_VPN) {
            if (resultCode == RESULT_OK) startVpn()
            else Toast.makeText(this, "Разрешение VPN не выдано", Toast.LENGTH_SHORT).show()
        }
    }

    private fun startVpn() {
        pending = true
        val i = Intent(this, VeilVpnService::class.java)
            .setAction(VeilVpnService.ACTION_CONNECT)
            .putExtra(VeilVpnService.EXTRA_LINK, Store.loadLink(this))
        startForegroundService(i)
    }

    private fun render() {
        val st = try { JSONObject(Veilcore.status()) } catch (_: Exception) { JSONObject() }
        val state = st.optString("state", "stopped")
        val up = st.optBoolean("up")
        if (state != "stopped") pending = false
        val active = state != "stopped" || pending
        connectBtn.text = if (active) "Отключить" else "Подключить"
        statusView.text = when {
            up -> "Подключено"
            state == "reconnecting" -> "Переподключение…"
            active -> "Подключение…"
            else -> "Отключено"
        }
        statusView.setTextColor(
            getColor(if (up) R.color.ok else if (active) R.color.text else R.color.muted)
        )
        val parts = listOf(st.optString("name"), st.optString("entry"), st.optString("transport"))
            .filter { it.isNotBlank() }
        serverView.text = parts.joinToString(" · ")
        trafficView.text = "↓ ${VeilVpnService.fmt(st.optLong("rx"))}   ↑ ${VeilVpnService.fmt(st.optLong("tx"))}"
        val err = if (state == "stopped") Store.lastError.orEmpty() else st.optString("last_error")
        errorView.text = err
        errorView.visibility = if (err.isBlank()) View.GONE else View.VISIBLE
    }

    companion object {
        private const val REQ_VPN = 1
    }
}
