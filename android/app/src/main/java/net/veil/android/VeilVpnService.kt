package net.veil.android

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.VpnService
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelFileDescriptor
import android.util.Log
import org.json.JSONObject
import veilcore.Protector
import veilcore.Veilcore

/**
 * VPN-сервис: Builder из Veilcore.parse(), TUN-дескриптор отдаётся Go-ядру, сокеты ядра защищаются protect().
 * Содержимое ссылки и ключи в журнал не пишутся.
 */
class VeilVpnService : VpnService() {

    private var tun: ParcelFileDescriptor? = null
    private val main = Handler(Looper.getMainLooper())
    @Volatile private var running = false

    private val ticker = object : Runnable {
        override fun run() {
            if (!running) return
            updateNotification()
            main.postDelayed(this, 2000)
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_CONNECT -> {
                val link = intent.getStringExtra(EXTRA_LINK)
                if (link.isNullOrBlank()) {
                    stopSelf()
                    return START_NOT_STICKY
                }
                startForegroundCompat(buildNotification("Подключение…"))
                Thread { connect(link) }.start()
            }
            ACTION_DISCONNECT -> disconnect()
            else -> {
                // перезапуск системой без интента: ссылку не храним в сервисе — просто останавливаемся
                if (!running) stopSelf()
            }
        }
        return START_NOT_STICKY
    }

    private fun connect(link: String) {
        synchronized(this) {
            if (running) return
            try {
                val p = JSONObject(Veilcore.parse(link))
                val b = Builder()
                    .setSession(p.optString("name", "Veil").ifBlank { "Veil" })
                    .setMtu(p.optInt("mtu", 1280))
                    .setBlocking(false)
                val addrs = p.getJSONArray("addresses")
                for (i in 0 until addrs.length()) {
                    val a = addrs.getJSONObject(i)
                    b.addAddress(a.getString("addr"), a.getInt("prefix"))
                }
                b.addRoute("0.0.0.0", 0)
                b.addRoute("::", 0)
                val dns = p.getJSONArray("dns")
                for (i in 0 until dns.length()) b.addDnsServer(dns.getString(i))
                val pfd = b.establish() ?: throw IllegalStateException("VPN не создан (нет разрешения?)")
                tun = pfd
                // дескриптором дальше владеет Go-ядро
                val fd = pfd.detachFd()
                Veilcore.start(link, fd, object : Protector {
                    override fun protect(fd: Int): Boolean = this@VeilVpnService.protect(fd)
                })
                running = true
                Store.lastError = null
                main.post(ticker)
                Log.i(TAG, "tunnel started")
            } catch (e: Exception) {
                Log.w(TAG, "connect failed: ${e.javaClass.simpleName}")
                Store.lastError = e.message ?: e.javaClass.simpleName
                closeAll()
                main.post { stopForegroundCompat(); stopSelf() }
            }
        }
    }

    private fun closeAll() {
        running = false
        try { Veilcore.stop() } catch (_: Exception) {}
        try { tun?.close() } catch (_: Exception) {}
        tun = null
    }

    private fun disconnect() {
        Thread {
            synchronized(this) { closeAll() }
            main.post { stopForegroundCompat(); stopSelf() }
        }.start()
    }

    override fun onRevoke() {
        // пользователь отключил VPN системой или запустил другой VPN
        disconnect()
    }

    override fun onDestroy() {
        main.removeCallbacksAndMessages(null)
        synchronized(this) { closeAll() }
        super.onDestroy()
    }

    // ---------- уведомление ----------

    private fun buildNotification(text: String): Notification {
        val nm = getSystemService(NotificationManager::class.java)
        if (nm.getNotificationChannel(CHANNEL) == null) {
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL, "Veil VPN", NotificationManager.IMPORTANCE_LOW)
            )
        }
        val open = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        val stop = PendingIntent.getService(
            this, 1, Intent(this, VeilVpnService::class.java).setAction(ACTION_DISCONNECT),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        return Notification.Builder(this, CHANNEL)
            .setSmallIcon(R.drawable.ic_notify)
            .setContentTitle("Veil")
            .setContentText(text)
            .setContentIntent(open)
            .setOngoing(true)
            .addAction(Notification.Action.Builder(null, "Отключить", stop).build())
            .build()
    }

    private fun updateNotification() {
        val st = try { JSONObject(Veilcore.status()) } catch (_: Exception) { return }
        val text = when {
            st.optBoolean("up") -> "Подключено · ↓ ${fmt(st.optLong("rx"))} ↑ ${fmt(st.optLong("tx"))}"
            st.optString("state") == "reconnecting" -> "Переподключение…"
            else -> "Подключение…"
        }
        getSystemService(NotificationManager::class.java).notify(NOTIF_ID, buildNotification(text))
    }

    private fun startForegroundCompat(n: Notification) {
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(NOTIF_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(NOTIF_ID, n)
        }
    }

    private fun stopForegroundCompat() {
        stopForeground(STOP_FOREGROUND_REMOVE)
    }

    companion object {
        const val TAG = "VeilVpn"
        const val ACTION_CONNECT = "net.veil.android.CONNECT"
        const val ACTION_DISCONNECT = "net.veil.android.DISCONNECT"
        const val EXTRA_LINK = "link"
        private const val CHANNEL = "veil_vpn"
        private const val NOTIF_ID = 1

        fun fmt(n: Long): String {
            val u = arrayOf("Б", "КБ", "МБ", "ГБ")
            var v = n.toDouble()
            var i = 0
            while (v >= 1024 && i < u.size - 1) { v /= 1024; i++ }
            return if (i == 0) "$n Б" else String.format("%.1f %s", v, u[i])
        }
    }
}
