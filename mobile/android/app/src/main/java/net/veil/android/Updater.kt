package net.veil.android

import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.net.Uri
import android.provider.Settings as AndroidSettings
import org.json.JSONObject
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest

/**
 * Обновление из GitHub-релизов: проверка последней версии, загрузка APK с проверкой SHA-256 (SHA256SUMS.txt того же релиза)
 * и установка через системный PackageInstaller (Android спросит подтверждение; подпись нового APK должна совпадать с установленным).
 * Файлы скачиваются только из релизов этого репозитория.
 */
object Updater {
    private const val API = "https://api.github.com/repos/dmitriy-lyakhov/Veil/releases/latest"
    private const val PREFIX = "https://github.com/dmitriy-lyakhov/Veil/releases/download/"
    const val ACTION = "net.veil.android.INSTALL_RESULT"

    class Release(val tag: String, val page: String, val assets: Map<String, String>) {
        val version get() = tag.removePrefix("v")
        /** Имя APK в релизе («Veil-<версия>-android.apk»). */
        val apk get() = assets.keys.firstOrNull { it.startsWith("Veil-") && it.endsWith("-android.apk") }
    }

    fun current(ctx: Context): String = try { ctx.packageManager.getPackageInfo(ctx.packageName, 0).versionName ?: "" } catch (_: Exception) { "" }

    /** latest новее current (сравнение по числам x.y.z). */
    fun newer(latest: String, current: String): Boolean {
        fun parse(s: String) = s.removePrefix("v").split(".", "-").take(3).map { it.toIntOrNull() ?: 0 }.let { it + List(3 - it.size) { 0 } }
        val a = parse(latest); val b = parse(current)
        for (i in 0 until 3) if (a[i] != b[i]) return a[i] > b[i]
        return false
    }

    private fun open(url: String): HttpURLConnection {
        val c = URL(url).openConnection() as HttpURLConnection
        c.connectTimeout = 15000; c.readTimeout = 30000
        c.setRequestProperty("User-Agent", "Veil-update")
        return c
    }

    /** Блокирующий запрос последнего релиза; вызывать не из потока интерфейса. */
    fun fetch(): Release {
        val c = open(API).also { it.setRequestProperty("Accept", "application/vnd.github+json") }
        try {
            if (c.responseCode != 200) throw IllegalStateException("GitHub: ${c.responseCode}")
            val j = JSONObject(c.inputStream.use { it.readBytes().take(4 shl 20).toByteArray().toString(Charsets.UTF_8) })
            val assets = mutableMapOf<String, String>()
            val arr = j.optJSONArray("assets")
            if (arr != null) for (i in 0 until arr.length()) {
                val a = arr.getJSONObject(i); assets[a.getString("name")] = a.getString("browser_download_url")
            }
            return Release(j.getString("tag_name"), j.optString("html_url"), assets)
        } finally { c.disconnect() }
    }

    /**
     * Скачивает APK релиза и проверяет SHA-256. progress(скачано, всего); cancelled() — прервать. Возвращает файл.
     * Блокирующий; вызывать не из потока интерфейса.
     */
    fun download(ctx: Context, rel: Release, progress: (Long, Long) -> Unit, cancelled: () -> Boolean): File {
        val name = rel.apk ?: throw IllegalStateException("E_NOAPK")
        val url = rel.assets[name]!!
        val sumsUrl = rel.assets["SHA256SUMS.txt"] ?: throw IllegalStateException("E_NOSUMS")
        if (!url.startsWith(PREFIX) || !sumsUrl.startsWith(PREFIX)) throw IllegalStateException("E_URL")
        val sums = open(sumsUrl).let { c -> try { c.inputStream.use { it.readBytes().take(64 shl 10).toByteArray().toString(Charsets.UTF_8) } } finally { c.disconnect() } }
        val want = sums.lines().map { it.trim().split(Regex("\\s+")) }
            .firstOrNull { it.size == 2 && it[1].removePrefix("*") == name }?.get(0)?.lowercase() ?: throw IllegalStateException("E_NOSUMS")
        val dir = File(ctx.cacheDir, "updates").apply { deleteRecursively(); mkdirs() }
        val out = File(dir, name)
        val md = MessageDigest.getInstance("SHA-256")
        val c = open(url)
        try {
            if (c.responseCode != 200) throw IllegalStateException("GitHub: ${c.responseCode}")
            val total = c.contentLengthLong
            c.inputStream.use { ins -> out.outputStream().use { os ->
                val buf = ByteArray(64 shl 10); var done = 0L
                while (true) {
                    if (cancelled()) throw InterruptedException()
                    val n = ins.read(buf); if (n < 0) break
                    os.write(buf, 0, n); md.update(buf, 0, n); done += n
                    progress(done, total)
                }
            } }
        } finally { c.disconnect() }
        val got = md.digest().joinToString("") { "%02x".format(it) }
        if (got != want) { out.delete(); throw IllegalStateException("E_SUM") }
        return out
    }

    /** Разрешено ли приложению ставить пакеты («Установка неизвестных приложений»); если нет — открывает нужный экран настроек. */
    fun ensureCanInstall(ctx: Context): Boolean {
        if (ctx.packageManager.canRequestPackageInstalls()) return true
        ctx.startActivity(Intent(AndroidSettings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:${ctx.packageName}")).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        return false
    }

    /** Отдаёт APK системному установщику; подтверждение показывает Android, итог приходит в InstallReceiver. */
    fun install(ctx: Context, apk: File) {
        val pi = ctx.packageManager.packageInstaller
        val id = pi.createSession(PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL))
        pi.openSession(id).use { s ->
            apk.inputStream().use { ins -> s.openWrite("veil.apk", 0, apk.length()).use { os -> ins.copyTo(os); s.fsync(os) } }
            val intent = Intent(ctx, InstallReceiver::class.java).setAction(ACTION)
            val pending = PendingIntent.getBroadcast(ctx, id, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_MUTABLE)
            s.commit(pending.intentSender)
        }
    }
}

/** Результат установки: просит подтверждение у пользователя, при ошибке показывает причину. */
class InstallReceiver : BroadcastReceiver() {
    override fun onReceive(c: Context, i: Intent) {
        when (i.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE)) {
            PackageInstaller.STATUS_PENDING_USER_ACTION -> {
                @Suppress("DEPRECATION")
                val confirm = i.getParcelableExtra<Intent>(Intent.EXTRA_INTENT)
                confirm?.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)?.let { c.startActivity(it) }
            }
            PackageInstaller.STATUS_SUCCESS -> {}
            else -> android.widget.Toast.makeText(c, c.getString(R.string.upd_failed, i.getStringExtra(PackageInstaller.EXTRA_STATUS_MESSAGE) ?: ""), android.widget.Toast.LENGTH_LONG).show()
        }
    }
}
