package net.veil.android

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.view.Gravity
import android.view.View
import android.widget.LinearLayout
import org.json.JSONArray
import org.json.JSONObject
import veilcore.InstallListener
import veilcore.Veilcore

// =============================== Свой сервер по SSH ===============================
// Дизайн: «Veil — Docker и связи сетей», артборды M1–M4. Пароль и ключ SSH живут только в памяти процесса
// (не пишутся на диск); файлы сервера сервер скачивает сам из GitHub-релиза версии приложения.

/** Данные формы между перестроениями страниц. */
object SshForm {
    var name = ""
    var host = ""
    var user = "root"
    var port = "22"
    var byKey = false
    var password = ""
    var key = ""
    var keyName = ""
    var passphrase = ""
    var proto = 0          // 0 — Veil, 1 — WireGuard, 2 — AmneziaWG
    var admin = true

    val protocols = listOf("veil", "wireguard", "amneziawg")

    /** Забыть секреты (после успешной установки или ухода с формы). */
    fun forgetSecrets() { password = ""; key = ""; keyName = ""; passphrase = "" }
}

/** Ход установки: состояние живёт вне страниц, чтобы установка шла и при свёрнутом приложении. */
object Installer {
    class Step(val id: String, var title: String, var detail: String, var state: String)

    val steps = mutableListOf<Step>()
    val log = StringBuilder()
    var running = false; private set
    var error: String? = null; private set
    var result: JSONObject? = null; private set
    var profileId: String? = null; private set
    var proto = "veil"; private set
    var server = ""; private set
    private var hostKeyId = ""
    /** Ошибка входа SSH (пароль или ключ): вместо «Повторить» — вернуться к форме. */
    val authError get() = error == "E_SSH_AUTH"
    /** Растёт при каждом изменении — страницы перестраиваются, когда видят новое значение. */
    @Volatile var version = 0; private set

    private val main = Handler(Looper.getMainLooper())

    fun start(ctx: Context, protoKey: String) {
        if (running) return
        val app = ctx.applicationContext
        steps.clear(); log.setLength(0); error = null; result = null; profileId = null
        proto = protoKey; server = SshForm.name.trim().ifEmpty { SshForm.host.trim() }
        running = true; version++
        val port = SshForm.port.trim().toIntOrNull() ?: 22
        hostKeyId = "ssh_hostkey:" + SshForm.host.trim().lowercase() + ":" + port
        val req = JSONObject()
            .put("name", server).put("host", SshForm.host.trim())
            .put("port", port).put("user", SshForm.user.trim().ifEmpty { "root" })
            .put("protocol", protoKey).put("admin", protoKey == "veil" && SshForm.admin)
            .put("client", Build.MODEL ?: "Android").put("version", Updater.current(app).substringBefore('-'))
            .put("hostkey", Settings.str(app, hostKeyId))
        if (SshForm.byKey) req.put("key", SshForm.key).put("passphrase", SshForm.passphrase) else req.put("password", SshForm.password)
        notify(app, app.getString(R.string.ssh_notif_running, server), true)
        Thread {
            var out: String? = null
            var err: String? = null
            try {
                out = Veilcore.install(req.toString(), object : InstallListener {
                    override fun onStep(id: String, title: String, detail: String, state: String) = main.post { step(app, id, title, detail, state) }.let { }
                    override fun onLog(line: String) = main.post { log.append(line).append('\n'); version++ }.let { }
                })
            } catch (e: Exception) { err = e.message ?: "?" }
            main.post { finish(app, out, err) }
        }.start()
    }

    fun cancel() { if (running) Veilcore.cancelInstall() }

    private fun step(ctx: Context, id: String, title: String, detail: String, state: String) {
        val s = steps.firstOrNull { it.id == id }
        if (s == null) steps.add(Step(id, title, detail, state)) else { s.title = title; s.detail = detail; s.state = state }
        log.append("› ").append(title).append(": ").append(detail).append('\n')
        if (running && state == "active") notify(ctx, ctx.getString(R.string.ssh_notif_step, server, title), true)
        version++
    }

    private fun finish(ctx: Context, out: String?, err: String?) {
        running = false
        if (err != null || out == null) {
            error = err ?: "?"
            steps.lastOrNull { it.state == "active" }?.state = "fail"
            // понятное объяснение — в карточке ошибки; сырой текст SSH остаётся в журнале
            if (error == "E_SSH_AUTH" || error == "E_SSH_HOSTKEY") steps.lastOrNull { it.state == "fail" }?.detail = ""
            log.append(ctx.getString(R.string.ssh_error_prefix)).append(error).append('\n')
            notify(ctx, ctx.getString(R.string.ssh_notif_failed, server), false)
        } else {
            val r = JSONObject(out)
            result = r
            // ключ хоста запоминается после первой удачной установки: при его смене следующая установка остановится
            r.optString("hostkey").takeIf { it.isNotEmpty() }?.let { Settings.set(ctx, hostKeyId, it) }
            try {
                val p = Profiles.addRaw(ctx, r.optString("config"), server)
                Settings.setActive(ctx, p.id)
                profileId = p.id
                notify(ctx, ctx.getString(R.string.ssh_notif_done, server), false)
            } catch (e: Exception) {
                error = e.message ?: "E_FORMAT"
                notify(ctx, ctx.getString(R.string.ssh_notif_failed, server), false)
            }
        }
        version++
    }

    private fun notify(ctx: Context, text: String, ongoing: Boolean) {
        val nm = ctx.getSystemService(NotificationManager::class.java) ?: return
        if (nm.getNotificationChannel(CHANNEL) == null) {
            nm.createNotificationChannel(NotificationChannel(CHANNEL, ctx.getString(R.string.ssh_notif_channel), NotificationManager.IMPORTANCE_LOW))
        }
        val open = PendingIntent.getActivity(ctx, 2, Intent(ctx, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        val n = Notification.Builder(ctx, CHANNEL)
            .setSmallIcon(R.drawable.ic_notify).setContentTitle(ctx.getString(R.string.ssh_title)).setContentText(text)
            .setContentIntent(open).setOngoing(ongoing).setAutoCancel(!ongoing).setOnlyAlertOnce(true)
        if (ongoing) n.setProgress(0, 0, true)
        try { nm.notify(NOTIF_ID, n.build()) } catch (_: SecurityException) {}
    }

    fun clearNotification(ctx: Context) {
        ctx.getSystemService(NotificationManager::class.java)?.cancel(NOTIF_ID)
    }

    private const val CHANNEL = "veil_install"
    private const val NOTIF_ID = 7
}

private fun protoTitle(a: Context, key: String) = when (key) {
    "wireguard" -> "WireGuard"
    "amneziawg" -> "AmneziaWG"
    else -> "Veil"
}

/** M2 — подключение по SSH и выбор протокола. */
class SshFormPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.ssh_title)

    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(16), dp(4), dp(16), dp(20)) }
        val gap = { v: View -> col.addView(v, lp(MATCH, WRAP).apply { bottomMargin = dp(8) }) }
        gap(a.field(a.getString(R.string.ssh_name), SshForm.name, a.getString(R.string.ssh_name_hint)) { SshForm.name = it }.first)
        gap(a.field(a.getString(R.string.ssh_host), SshForm.host, a.getString(R.string.ssh_host_hint)) { SshForm.host = it }.first)
        val pair = a.hbox()
        pair.addView(a.field(a.getString(R.string.ssh_user), SshForm.user) { SshForm.user = it }.first, lp(0, WRAP, 1f).apply { rightMargin = dp(8) })
        pair.addView(a.field(a.getString(R.string.ssh_port), SshForm.port, numeric = true) { SshForm.port = it }.first, lp(dp(110), WRAP))
        gap(pair)

        col.addView(section(R.string.ssh_auth))
        gap(a.segmented(listOf(a.getString(R.string.ssh_by_password), a.getString(R.string.ssh_by_key)), if (SshForm.byKey) 1 else 0) {
            SshForm.byKey = it == 1; a.rebuild()
        })
        if (!SshForm.byKey) {
            gap(a.field(a.getString(R.string.ssh_password), SshForm.password, secret = true) { SshForm.password = it }.first)
        } else {
            val sub = if (SshForm.keyName.isEmpty()) a.getString(R.string.ssh_key_pick_sub) else a.getString(R.string.ssh_key_replace)
            gap(a.row(SshForm.keyName.ifEmpty { a.getString(R.string.ssh_key_pick) }, sub, R.drawable.ic_shield) { a.pickSshKey() })
            gap(a.field(a.getString(R.string.ssh_passphrase), SshForm.passphrase, a.getString(R.string.ssh_passphrase_hint), secret = true) { SshForm.passphrase = it }.first)
        }

        col.addView(section(R.string.ssh_protocol))
        val subs = listOf(R.string.ssh_proto_veil, R.string.ssh_proto_wg, R.string.ssh_proto_awg)
        SshForm.protocols.forEachIndexed { i, key ->
            val on = SshForm.proto == i
            val r = a.row(protoTitle(a, key), a.getString(subs[i]), null, a.radio(on)) { SshForm.proto = i; a.rebuild() }
            r.background = rounded(T.p.raised, 14f, if (on) T.p.accent else 0, if (on) 2f else 0f)
            gap(r)
        }
        if (SshForm.proto == 0) {
            gap(a.row(a.getString(R.string.ssh_admin), a.getString(R.string.ssh_admin_sub), null, Toggle(a, SshForm.admin) { SshForm.admin = it }) {})
        }
        val ver = Updater.current(a).substringBefore('-')
        val hint = if (SshForm.byKey) a.getString(R.string.ssh_hint_key, ver) else a.getString(R.string.ssh_hint_password, ver)
        col.addView(a.noteCard(hint, R.drawable.ic_info), lp(MATCH, WRAP).apply { topMargin = dp(4); bottomMargin = dp(16) })
        col.addView(a.button(a.getString(R.string.ssh_install)) { submit() }, lp(MATCH, WRAP))
        return col
    }

    private fun section(res: Int) = a.text(a.getString(res), 12f, 600, T.p.muted).also { it.setPadding(dp(4), dp(12), 0, dp(8)) }

    private fun submit() {
        val host = SshForm.host.trim()
        val port = SshForm.port.trim().toIntOrNull()
        when {
            host.isEmpty() -> { a.toast(a.getString(R.string.ssh_err_host)); return }
            port == null || port !in 1..65535 -> { a.toast(a.getString(R.string.ssh_err_port)); return }
            !SshForm.byKey && SshForm.password.isEmpty() -> { a.toast(a.getString(R.string.ssh_err_password)); return }
            SshForm.byKey && SshForm.key.isBlank() -> { a.toast(a.getString(R.string.ssh_err_key)); return }
        }
        Installer.start(a, SshForm.protocols[SshForm.proto])
        a.push(InstallPage(a))
    }
}

/** M3 — ход установки. */
class InstallPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.ssh_install_title, Installer.server)
    private var seen = -1

    override fun refresh() {
        if (Installer.version != seen) {
            // протокол уже стоял — экран «Сервер уже настроен» (M4) вместо этого
            if (!Installer.running && Installer.error == null && Installer.result?.optBoolean("existing") == true) {
                seen = Installer.version
                a.pop(); a.push(ExistingPage(a)); return
            }
            a.rebuild()
        }
    }

    override fun build(): View {
        seen = Installer.version
        val col = a.vbox().apply { setPadding(dp(16), dp(0), dp(16), dp(20)) }
        val who = SshForm.user.ifEmpty { "root" } + "@" + SshForm.host.trim()
        val adminTxt = if (Installer.proto == "veil") " · " + a.getString(if (SshForm.admin) R.string.ssh_admin_yes else R.string.ssh_admin_no) else ""
        col.addView(a.text(protoTitle(a, Installer.proto) + " · " + who + adminTxt, 13f, 500, T.p.muted).also { it.gravity = Gravity.CENTER; it.setPadding(0, 0, 0, dp(10)) }, lp(MATCH, WRAP))

        // полоса хода: доля завершённых шагов из ожидаемых (9 у Veil в Docker, 6 у WireGuard/AmneziaWG)
        val total = if (Installer.proto == "veil") 8 else 6
        val done = Installer.steps.count { it.state == "done" || it.state == "skip" }
        val frac = when { !Installer.running && Installer.error == null -> 1f; else -> (done.toFloat() / total).coerceIn(0.04f, 0.96f) }
        val track = LinearLayout(a).apply { background = rounded(T.p.high, 3f) }
        track.addView(View(a).apply { background = rounded(T.p.accent, 3f) }, LinearLayout.LayoutParams(0, dp(6), frac))
        track.addView(View(a), LinearLayout.LayoutParams(0, dp(6), 1f - frac))
        col.addView(track, lp(MATCH, dp(6)).apply { bottomMargin = dp(10) })

        for (s in Installer.steps) col.addView(stepRow(s), lp(MATCH, WRAP).apply { bottomMargin = dp(6) })
        if (Installer.running && Installer.steps.none { it.state == "active" }) {
            col.addView(a.text(a.getString(R.string.ssh_working), 13f, 500, T.p.muted).also { it.setPadding(dp(4), dp(4), 0, 0) })
        }

        val err = Installer.error
        if (err != null) {
            val box = a.vbox().apply { setPadding(dp(14), dp(14), dp(14), dp(14)); background = rounded(T.p.raised, 14f, T.p.accent, 2f) }
            box.addView(a.text(a.getString(R.string.ssh_failed), 15f, 600))
            box.addView(a.text(a.errorText(err), 13f, 500, T.p.muted).also { it.setPadding(0, dp(6), 0, 0); it.setLineSpacing(0f, 1.25f) })
            col.addView(box, lp(MATCH, WRAP).apply { topMargin = dp(4) })
        }
        if (Installer.running) {
            col.addView(a.text(a.getString(R.string.ssh_background), 12f, 500, T.p.muted).also { it.gravity = Gravity.CENTER; it.setPadding(dp(12), dp(10), dp(12), 0) }, lp(MATCH, WRAP))
        }

        col.addView(a.space(16f))
        when {
            !Installer.running && err == null -> col.addView(a.button(a.getString(R.string.ssh_connect)) { connect() }, lp(MATCH, WRAP))
            // неверный пароль или ключ: повтор с теми же данными бесполезен — назад к форме
            Installer.authError -> col.addView(a.button(a.getString(R.string.ssh_edit)) { SshForm.password = ""; a.pop() }, lp(MATCH, WRAP))
            err != null -> col.addView(a.button(a.getString(R.string.ssh_retry)) { Installer.start(a, Installer.proto); a.rebuild() }, lp(MATCH, WRAP))
        }
        val bottom = a.hbox()
        if (Installer.running) {
            bottom.addView(a.button(a.getString(R.string.ssh_log), primary = false) { showLog() }, lp(0, WRAP, 1f).apply { rightMargin = dp(8) })
            bottom.addView(a.button(a.getString(R.string.ssh_cancel), primary = false) { Installer.cancel() }, lp(0, WRAP, 1f))
        } else {
            bottom.addView(a.button(a.getString(R.string.ssh_log), primary = false) { showLog() }, lp(0, WRAP, 1f))
        }
        col.addView(bottom, lp(MATCH, WRAP).apply { topMargin = dp(8) })
        return col
    }

    private fun stepRow(s: Installer.Step): View {
        val r = a.hbox().apply {
            setPadding(dp(14), dp(8), dp(14), dp(8)); minimumHeight = dp(56)
            background = rounded(if (s.state == "active" || s.state == "fail") T.p.high else T.p.raised, 14f)
        }
        val mark: View = when (s.state) {
            "done" -> a.icon(R.drawable.ic_check, 22f, T.p.success)
            "fail" -> a.icon(R.drawable.ic_close, 22f, T.p.accentInk)
            "skip" -> a.text("—", 18f, 600, T.p.muted).also { it.gravity = Gravity.CENTER }
            else -> android.widget.ProgressBar(a).apply { isIndeterminate = true; indeterminateTintList = android.content.res.ColorStateList.valueOf(T.p.accent) }
        }
        r.addView(mark, LinearLayout.LayoutParams(dp(22), dp(22)).apply { rightMargin = dp(12) })
        val txt = a.vbox()
        txt.addView(a.text(s.title, 15f, 600))
        if (s.detail.isNotEmpty()) txt.addView(a.text(s.detail, 12f, 500, T.p.muted, lines = 3).also { it.setPadding(0, dp(2), 0, 0) })
        r.addView(txt, lp(0, WRAP, 1f))
        return r
    }

    private fun showLog() {
        val body = a.vbox()
        val t = a.text(Installer.log.toString().ifBlank { "—" }, 11f, 500, T.p.muted)
        t.typeface = android.graphics.Typeface.MONOSPACE; t.setTextIsSelectable(true)
        t.background = rounded(T.p.surface, 12f); t.setPadding(dp(12), dp(12), dp(12), dp(12))
        val sc = android.widget.ScrollView(a).apply { addView(t); isVerticalScrollBarEnabled = true }
        body.addView(sc, lp(MATCH, dp(320)))
        body.addView(a.button(a.getString(R.string.share), primary = false) {
            a.startActivity(Intent.createChooser(Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, Installer.log.toString()), null))
        }, lp(MATCH, WRAP).apply { topMargin = dp(12) })
        a.openSheet(a.getString(R.string.ssh_log), body)
    }

    private fun connect() {
        Installer.clearNotification(a)
        SshForm.forgetSecrets()
        val id = Installer.profileId
        a.selectTab(0)
        if (id != null) a.connect(id)
    }
}

/** M4 — протокол уже стоял на сервере: выдан только новый клиент. */
class ExistingPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.ssh_existing_title)
    private var seen = Installer.version

    override fun refresh() {
        // нажали другой протокол — пошла новая установка: показываем её ход
        if (Installer.version != seen && Installer.running) { seen = Installer.version; a.pop(); a.push(InstallPage(a)) }
    }

    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(16), dp(4), dp(16), dp(20)) }
        val r = Installer.result ?: JSONObject()
        val client = r.optString("client").ifEmpty { Build.MODEL ?: "" }
        val card = a.vbox().apply { setPadding(dp(16), dp(16), dp(16), dp(16)); background = rounded(T.p.raised, 16f) }
        val head = a.hbox()
        head.addView(a.icon(R.drawable.ic_check, 24f, T.p.success).also { (it.layoutParams as LinearLayout.LayoutParams).rightMargin = dp(12) })
        val ht = a.vbox()
        ht.addView(a.text(a.getString(R.string.ssh_existing_client, protoTitle(a, Installer.proto), client), 17f, 600))
        val sub = if (Installer.proto == "veil") a.getString(R.string.ssh_existing_admin, Installer.server) else a.getString(R.string.ssh_existing_profile, protoTitle(a, Installer.proto), Installer.server)
        ht.addView(a.text(sub, 13f, 500, T.p.muted).also { it.setPadding(0, dp(2), 0, 0) })
        head.addView(ht, lp(0, WRAP, 1f))
        card.addView(head, lp(MATCH, WRAP))
        card.addView(a.text(a.getString(R.string.ssh_existing_note), 13f, 500, T.p.muted).also { it.setPadding(0, dp(10), 0, 0); it.setLineSpacing(0f, 1.25f) })
        col.addView(card, lp(MATCH, WRAP))

        val list = r.optJSONArray("containers") ?: JSONArray()
        var others = false
        if (list.length() > 0) {
            col.addView(a.text(a.getString(R.string.ssh_existing_running), 12f, 600, T.p.muted).also { it.setPadding(dp(4), dp(16), 0, dp(8)) })
            for (i in 0 until list.length()) {
                val c = list.getJSONObject(i)
                val proto = c.optString("protocol")
                val canIssue = proto in SshForm.protocols && proto != Installer.proto
                val dot = View(a).apply { background = rounded(if (c.optString("state") == "running") T.p.success else T.p.muted, 5f) }
                val row = a.hbox().apply { setPadding(dp(16), dp(10), dp(16), dp(10)); minimumHeight = dp(64); ripple(rounded(T.p.raised, 14f)) }
                row.addView(dot, LinearLayout.LayoutParams(dp(10), dp(10)).apply { rightMargin = dp(12) })
                val t = a.vbox()
                t.addView(a.text(c.optString("name"), 16f, 600))
                // панель живёт в сети контейнера veil: его порты к ней не относятся
                val ports = if (proto == "admin") a.getString(R.string.ssh_no_ports) else shortPorts(c.optString("ports"))
                t.addView(a.text(listOf(protoName(proto), ports).filter { it.isNotEmpty() }.joinToString(" · "), 13f, 500, T.p.muted).also { it.setPadding(0, dp(3), 0, 0) })
                row.addView(t, lp(0, WRAP, 1f))
                if (canIssue) {
                    others = true
                    row.addView(a.icon(R.drawable.ic_right, 20f, T.p.muted))
                    row.setOnClickListener { issue(proto) }
                }
                col.addView(row, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
            }
        }
        val canMore = others && (SshForm.password.isNotEmpty() || SshForm.key.isNotBlank())
        col.addView(a.noteCard(a.getString(if (canMore) R.string.ssh_existing_hint else R.string.ssh_existing_hint_short), R.drawable.ic_info), lp(MATCH, WRAP).apply { topMargin = dp(4); bottomMargin = dp(16) })
        col.addView(a.button(a.getString(R.string.ssh_connect)) {
            Installer.clearNotification(a)
            SshForm.forgetSecrets()
            val id = Installer.profileId
            a.selectTab(0)
            if (id != null) a.connect(id)
        }, lp(MATCH, WRAP))
        return col
    }

    /** «0.0.0.0:443->443/tcp, [::]:443->443/tcp, …» → «443/tcp, 443/udp, 80/tcp» (опубликованные порты хоста, без повторов). */
    private fun shortPorts(raw: String): String =
        Regex("""(?:^|[\s,])(?:[\d.]+|\[[0-9a-fA-F:]*\]):(\d+)->\d+/(tcp|udp)""").findAll(raw)
            .map { it.groupValues[1] + "/" + it.groupValues[2] }.distinct()
            .sortedWith(compareBy({ it != "443/tcp" && it != "443/udp" }, { it }))
            .joinToString(", ")

    private fun protoName(p: String) = when (p) {
        "veil" -> "Veil"; "admin" -> a.getString(R.string.ssh_admin); "wireguard" -> "WireGuard"; "amneziawg" -> "AmneziaWG"
        "openvpn" -> "OpenVPN"; "ipsec" -> "IPsec"; else -> p
    }

    /** Выдать клиента и для другого протокола на этом же сервере (нужны те же данные входа в памяти). */
    private fun issue(proto: String) {
        if (SshForm.password.isEmpty() && SshForm.key.isBlank()) { a.pop(); a.push(SshFormPage(a)); return }
        SshForm.proto = SshForm.protocols.indexOf(proto)
        Installer.start(a, proto)
        a.pop(); a.push(InstallPage(a))
    }
}
