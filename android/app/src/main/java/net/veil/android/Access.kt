package net.veil.android

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.text.Editable
import android.text.TextWatcher
import android.util.Base64
import android.view.Gravity
import android.view.View
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.LinearLayout
import org.json.JSONObject
import veilcore.Veilcore

/**
 * Вкладка «Доступ»: управление клиентами сервера через админ-API (`/admin/api/...`). Работает с профилем роли «Администратор»
 * при поднятом туннеле: запросы идёт по туннелю (Go-ядро, `Veilcore.admin`), сервер узнаёт администратора по адресу в туннеле.
 */
object Access {
    @Volatile var state: JSONObject? = null
    @Volatile var loading = false
    @Volatile var error: String? = null
    private var forId: String? = null

    /** Сведения о профиле из Go-ядра (роль, адрес админки и т. д.). */
    fun info(a: Context): JSONObject? = try {
        Profiles.active(a)?.let { JSONObject(Veilcore.parse(it.link)) }
    } catch (_: Exception) { null }

    /** Данные загружены для активного профиля. */
    fun ready(a: Context): Boolean = state != null && forId != null && forId == Profiles.active(a)?.id

    fun errorText(a: Context, code: String?): String = when (code) {
        null -> ""
        "E_NOADMIN" -> a.getString(R.string.ac_err_noadmin)
        else -> code
    }

    /** Запрос к админ-API в фоне; done вызывается в потоке интерфейса: (ответ, ошибка). */
    fun call(a: MainActivity, method: String, path: String, body: JSONObject?, done: (JSONObject?, String?) -> Unit) {
        val link = Profiles.active(a)?.link ?: return done(null, "E_EMPTY")
        Thread {
            var res: JSONObject? = null
            var err: String? = null
            try { res = JSONObject(Veilcore.admin(link, method, path, body?.toString() ?: "")) } catch (e: Exception) { err = e.message ?: "?" }
            a.runOnUiThread { done(res, err) }
        }.start()
    }

    /** Загрузить список клиентов (GET state). По готовности перестраивает текущую страницу. */
    fun load(a: MainActivity, force: Boolean = false) {
        val p = Profiles.active(a) ?: return
        if (loading || (!force && ready(a))) return
        loading = true; error = null
        call(a, "GET", "state", null) { r, e ->
            loading = false; forId = p.id; state = r; error = e
            a.rebuild()
        }
    }

    fun clients(): List<JSONObject> {
        val arr = state?.optJSONArray("clients") ?: return emptyList()
        val out = mutableListOf<JSONObject>()
        for (i in 0 until arr.length()) {
            val c = arr.getJSONObject(i)
            if (c.optString("relay_of").isEmpty()) out += c   // ключи каскада здесь не показываем
        }
        return out
    }

    fun find(name: String): JSONObject? = clients().firstOrNull { it.optString("name") == name }
}

/** Состояние интерфейса вкладки между перестроениями страницы. */
private object AccessUi {
    var tab = 0
    var name: String? = null
}

// ---------- общие элементы ----------

/** Карточка-пояснение со значком (дизайн: поля 14, текст 14/500). */
private fun MainActivity.noteCard(text: String, icon: Int, warn: Boolean = false): View {
    val box = hbox().apply { setPadding(dp(14), dp(14), dp(14), dp(14)); background = rounded(T.p.raised, 14f); gravity = Gravity.TOP }
    box.addView(icon(icon, 22f, if (warn) T.p.accentInk else T.p.muted).also { (it.layoutParams as LinearLayout.LayoutParams).rightMargin = dp(12) })
    box.addView(text(text, 14f, 500, T.p.muted).also { it.setLineSpacing(0f, 1.25f) }, lp(0, WRAP, 1f))
    return box
}

/** Строка «подпись — значение». */
private fun MainActivity.kv(label: String, value: String): View {
    val r = hbox().apply { setPadding(dp(16), dp(10), dp(16), dp(10)); minimumHeight = dp(52); background = rounded(T.p.raised, 14f) }
    r.addView(text(label, 13f, 500, T.p.muted))
    r.addView(text(value, 15f, 600).also { it.gravity = Gravity.END }, lp(0, WRAP, 1f).apply { leftMargin = dp(12) })
    return r
}

/** Поле ввода-карточка: подпись 12/500 и текст 16/600 (высота ≥ 64). */
private fun MainActivity.field(label: String, value: String, hint: String = "", bg: Int = T.p.raised, onChange: (String) -> Unit): Pair<View, EditText> {
    val card = vbox().apply { setPadding(dp(16), dp(10), dp(16), dp(10)); minimumHeight = dp(64); background = rounded(bg, 14f); gravity = Gravity.CENTER_VERTICAL }
    card.addView(text(label, 12f, 500, T.p.muted))
    val et = EditText(this).apply {
        setText(value); this.hint = hint
        setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, 16f)
        typeface = T.semibold; setTextColor(T.p.ink); setHintTextColor(T.p.muted)
        background = null; setPadding(0, dp(4), 0, 0); isSingleLine = true
        addTextChangedListener(object : TextWatcher {
            override fun afterTextChanged(s: Editable?) { onChange(s?.toString() ?: "") }
            override fun beforeTextChanged(s: CharSequence?, st: Int, c: Int, af: Int) {}
            override fun onTextChanged(s: CharSequence?, st: Int, b: Int, c: Int) {}
        })
    }
    card.addView(et, lp(MATCH, WRAP))
    return card to et
}

private fun fmtRoute(a: Context, c: JSONObject): String {
    val auto = c.optJSONObject("auto")
    val route = c.optJSONArray("route")
    return when {
        auto != null -> a.getString(R.string.ac_auto_fmt, auto.optString("exit"))
        route != null && route.length() > 0 -> (0 until route.length()).joinToString(" → ") { route.getString(it) }
        else -> a.getString(R.string.ac_exit_here)
    }
}

private fun fmtTraffic(c: JSONObject) = "↓ ${fmtBytes(c.optLong("rx_bytes"))}  ↑ ${fmtBytes(c.optLong("tx_bytes"))}"

private fun qrBitmap(dataUri: String?): Bitmap? = try {
    val raw = Base64.decode(dataUri!!.substringAfter("base64,"), Base64.DEFAULT)
    BitmapFactory.decodeByteArray(raw, 0, raw.size)
} catch (_: Exception) { null }

private fun roleName(a: Context, role: String) = a.getString(if (role == "admin") R.string.ac_role_admin else R.string.ac_role_user)

// ======================================= Доступ (вкладка) =======================================

class AccessPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.ac_title)
    private var gate = 0

    /** 0 — можно; 1 — нужна роль администратора; 2 — нужно подключение. */
    private fun gateState(): Int {
        if (Access.info(a)?.optString("role") != "admin") return 1
        val st = a.status
        return if (st.optString("state") == "connected" && st.optBoolean("up")) 0 else 2
    }

    override fun refresh() { if (gateState() != gate) a.rebuild() }

    override fun build(): View {
        gate = gateState()
        val col = a.vbox().apply { setPadding(dp(16), dp(4), dp(16), dp(16)) }
        if (gate != 0) {
            col.addView(a.noteCard(a.getString(if (gate == 1) R.string.ac_need_admin else R.string.ac_need_connect), R.drawable.ic_info))
            return col
        }
        Access.load(a)
        col.addView(a.segmented(listOf(a.getString(R.string.ac_seg_conn), a.getString(R.string.ac_seg_users)), AccessUi.tab) { AccessUi.tab = it; a.rebuild() }, lp(MATCH, WRAP))
        if (AccessUi.tab == 0) connectionTab(col) else usersTab(col)
        return col
    }

    private fun connectionTab(col: LinearLayout) {
        col.addView(a.text(a.getString(R.string.ac_lead), 14f, 500, T.p.muted).also { it.setPadding(dp(4), dp(14), 0, dp(12)); it.setLineSpacing(0f, 1.25f) })
        val (card, _) = a.field(a.getString(R.string.ac_user_name), AccessUi.name ?: a.getString(R.string.ac_default_name)) { AccessUi.name = it }
        col.addView(card, lp(MATCH, WRAP))
        var busy = false
        val share = a.button(a.getString(R.string.ac_share)) {}
        share.setOnClickListener {
            if (busy) return@setOnClickListener
            busy = true; share.text = a.getString(R.string.ac_loading)
            val name = (AccessUi.name ?: "").trim().ifEmpty { a.getString(R.string.ac_default_name) }
            Access.call(a, "POST", "clients", JSONObject().put("name", name).put("role", "user")) { r, e ->
                busy = false; share.text = a.getString(R.string.ac_share)
                if (e != null || r == null) { a.toast(Access.errorText(a, e)); return@call }
                AccessUi.name = null
                Access.load(a, true)
                a.push(ShareResultPage(a, r, "user"))
            }
        }
        col.addView(share, lp(MATCH, WRAP).apply { topMargin = dp(16) })
    }

    private fun usersTab(col: LinearLayout) {
        val head = a.hbox()
        head.addView(a.text(a.getString(R.string.ac_users), 20f, 600), lp(0, WRAP, 1f))
        val plus = a.icon(R.drawable.ic_plus, 22f).apply { setPadding(dp(11), dp(11), dp(11), dp(11)); setOnClickListener { a.push(NewClientPage(a)) } }
        plus.layoutParams = LinearLayout.LayoutParams(dp(44), dp(44))
        head.addView(plus)
        col.addView(head, lp(MATCH, WRAP).apply { topMargin = dp(8); bottomMargin = dp(4) })
        val err = Access.error
        when {
            err != null && !Access.loading -> {
                col.addView(a.noteCard(Access.errorText(a, err), R.drawable.ic_info, warn = true))
                col.addView(a.button(a.getString(R.string.ac_retry), primary = false) { Access.load(a, true) }, lp(MATCH, WRAP).apply { topMargin = dp(12) })
            }
            !Access.ready(a) -> col.addView(a.text(a.getString(R.string.ac_loading), 14f, 500, T.p.muted).also { it.setPadding(dp(4), dp(12), 0, 0) })
            Access.clients().isEmpty() -> col.addView(a.text(a.getString(R.string.ac_no_users), 14f, 500, T.p.muted).also { it.setPadding(dp(4), dp(12), 0, 0) })
            else -> for (c in Access.clients()) col.addView(clientRow(c), lp(MATCH, WRAP).apply { topMargin = dp(8) })
        }
    }

    private fun clientRow(c: JSONObject): View {
        val name = c.optString("name")
        val row = a.hbox().apply {
            setPadding(dp(16), dp(10), dp(16), dp(10)); minimumHeight = dp(56)
            ripple(rounded(T.p.raised, 14f)); setOnClickListener { a.push(ClientPage(a, name)) }
        }
        val left = a.vbox()
        val l1 = a.hbox()
        l1.addView(a.text(name, 15f, 600, lines = 1))
        if (c.optString("role") == "admin") {
            val badge = a.text(a.getString(R.string.ac_badge_admin), 11f, 600, T.p.accentInk).apply {
                background = rounded(Color.TRANSPARENT, 6f, T.p.accentInk, 1f); setPadding(dp(6), dp(1), dp(6), dp(1))
            }
            l1.addView(badge, lp(WRAP, WRAP).apply { leftMargin = dp(8) })
        }
        left.addView(l1)
        val route = fmtRoute(a, c)
        val sub = (if (c.optBoolean("disabled")) a.getString(R.string.ac_blocked) + " · " + route else route) + "\n" + fmtTraffic(c)
        left.addView(a.text(sub, 12f, 500, T.p.muted).also { it.setPadding(0, dp(4), 0, 0) })
        row.addView(left, lp(0, WRAP, 1f))
        row.addView(a.icon(R.drawable.ic_right, 20f, T.p.muted))
        return row
    }
}

// ======================================= Новый клиент =======================================

class NewClientPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.ac_new_client)
    private var name = ""
    private var note = ""
    private var role = "user"
    private var busy = false

    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(16), dp(4), dp(16), dp(16)) }
        col.addView(a.field(a.getString(R.string.ac_name), name, a.getString(R.string.ac_name_hint)) { name = it }.first, lp(MATCH, WRAP))
        col.addView(a.field(a.getString(R.string.ac_note), note) { note = it }.first, lp(MATCH, WRAP).apply { topMargin = dp(8) })
        col.addView(a.text(a.getString(R.string.ac_role), 12f, 600, T.p.muted).also { it.setPadding(dp(4), dp(16), 0, dp(8)) })
        col.addView(a.segmented(listOf(a.getString(R.string.ac_role_user), a.getString(R.string.ac_role_admin)), if (role == "admin") 1 else 0) {
            role = if (it == 1) "admin" else "user"; a.rebuild()
        }, lp(MATCH, WRAP))
        val admin = role == "admin"
        col.addView(a.noteCard(a.getString(if (admin) R.string.ac_role_note_admin else R.string.ac_role_note_user), if (admin) R.drawable.ic_info else R.drawable.ic_info, warn = admin),
            lp(MATCH, WRAP).apply { topMargin = dp(12) })
        val create = a.button(a.getString(R.string.ac_create)) {}
        create.setOnClickListener {
            if (busy) return@setOnClickListener
            val n = name.trim()
            if (n.isEmpty()) { a.toast(a.getString(R.string.err_empty)); return@setOnClickListener }
            busy = true; create.text = a.getString(R.string.ac_loading)
            Access.call(a, "POST", "clients", JSONObject().put("name", n).put("note", note.trim()).put("role", role)) { r, e ->
                busy = false; create.text = a.getString(R.string.ac_create)
                if (e != null || r == null) { a.toast(Access.errorText(a, e)); return@call }
                Access.load(a, true)
                a.pop()
                a.push(ShareResultPage(a, r, role))
            }
        }
        col.addView(create, lp(MATCH, WRAP).apply { topMargin = dp(16) })
        col.addView(a.button(a.getString(R.string.ac_cancel), primary = false) { a.pop() }, lp(MATCH, WRAP).apply { topMargin = dp(8) })
        return col
    }
}

// ======================================= Результат: QR и ссылка =======================================

class ShareResultPage(a: MainActivity, private val resp: JSONObject, private val role: String) : Page(a) {
    override val title get() = a.getString(R.string.ac_client_fmt, resp.optString("name"))

    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(16), dp(4), dp(16), dp(16)) }
        val link = resp.optString("link")
        col.addView(a.noteCard(a.getString(R.string.ac_share_note), R.drawable.ic_qr))
        qrBitmap(resp.optString("qr"))?.let { bmp ->
            // QR всегда на белом: тёмная тема не должна его инвертировать
            val box = FrameLayout(a).apply { background = rounded(Color.WHITE, 16f); setPadding(dp(14), dp(14), dp(14), dp(14)) }
            val iv = ImageView(a).apply { setImageBitmap(bmp); scaleType = ImageView.ScaleType.FIT_CENTER }
            box.addView(iv, FrameLayout.LayoutParams(dp(220), dp(220)))
            col.addView(box, lp(WRAP, WRAP).apply { gravity = Gravity.CENTER_HORIZONTAL; topMargin = dp(16); bottomMargin = dp(16) })
        }
        col.addView(a.kv(a.getString(R.string.ac_route), resp.optString("via").ifEmpty { a.getString(R.string.ac_exit_here) }), lp(MATCH, WRAP))
        col.addView(a.kv(a.getString(R.string.ac_role), roleName(a, role)), lp(MATCH, WRAP).apply { topMargin = dp(8) })
        col.addView(a.kv(a.getString(R.string.ac_link), if (link.length > 16) link.take(14) + "…" else link), lp(MATCH, WRAP).apply { topMargin = dp(8) })
        col.addView(a.button(a.getString(R.string.ac_copy)) { a.copyText(link) }, lp(MATCH, WRAP).apply { topMargin = dp(16) })
        col.addView(a.button(a.getString(R.string.ac_file), primary = false) { a.saveText(resp.optString("file").ifEmpty { "client.veil" }, link) }, lp(MATCH, WRAP).apply { topMargin = dp(8) })
        col.addView(a.button(a.getString(R.string.ac_add_here), primary = false) { a.importText(link, null) }, lp(MATCH, WRAP).apply { topMargin = dp(8) })
        col.addView(a.noteCard(a.getString(R.string.ac_key_once), R.drawable.ic_info), lp(MATCH, WRAP).apply { topMargin = dp(12) })
        return col
    }
}

// ======================================= Клиент =======================================

class ClientPage(a: MainActivity, private var name: String) : Page(a) {
    override val title get() = name

    private fun mutate(path: String, body: JSONObject, after: () -> Unit) {
        Access.call(a, "POST", path, body) { _, e ->
            if (e != null) a.toast(Access.errorText(a, e)) else { Access.load(a, true); after() }
        }
    }

    private fun action(title: String, sub: String, danger: Boolean = false, onClick: () -> Unit): View {
        val row = a.row(title, sub.ifEmpty { null }, null, onClick = onClick)
        if (danger) ((row.getChildAt(0) as LinearLayout).getChildAt(0) as android.widget.TextView).setTextColor(T.p.accentInk)
        return row
    }

    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(16), dp(4), dp(16), dp(16)) }
        val c = Access.find(name)
        if (c == null) {
            col.addView(a.text(a.getString(R.string.ac_loading), 14f, 500, T.p.muted).also { it.setPadding(dp(4), dp(12), 0, 0) })
            return col
        }
        val admin = c.optString("role") == "admin"
        val blocked = c.optBoolean("disabled")
        col.addView(a.kv(a.getString(R.string.ac_role), roleName(a, c.optString("role"))), lp(MATCH, WRAP))
        col.addView(a.kv(a.getString(R.string.ac_route), fmtRoute(a, c)), lp(MATCH, WRAP).apply { topMargin = dp(8) })
        col.addView(a.kv(a.getString(R.string.ac_traffic), fmtTraffic(c)), lp(MATCH, WRAP).apply { topMargin = dp(8) })
        val note = c.optString("note")
        if (note.isNotEmpty()) col.addView(a.kv(a.getString(R.string.ac_note), note), lp(MATCH, WRAP).apply { topMargin = dp(8) })
        val status = if (blocked) R.string.ac_blocked else if (c.optBoolean("online")) R.string.ac_online else R.string.ac_offline
        col.addView(a.kv(a.getString(R.string.ac_status), a.getString(status)), lp(MATCH, WRAP).apply { topMargin = dp(8) })
        col.addView(a.space(8f))
        fun add(v: View) = col.addView(v, lp(MATCH, WRAP).apply { topMargin = dp(8) })
        add(action(a.getString(if (admin) R.string.ac_make_user else R.string.ac_make_admin), a.getString(if (admin) R.string.ac_make_user_sub else R.string.ac_make_admin_sub)) {
            mutate("clients/update", JSONObject().put("name", name).put("role", if (admin) "user" else "admin")) {}
        })
        add(action(a.getString(R.string.ac_new_config), a.getString(R.string.ac_new_config_sub)) {
            Access.call(a, "POST", "clients/reissue", JSONObject().put("name", name)) { r, e ->
                if (e != null || r == null) a.toast(Access.errorText(a, e)) else a.push(ShareResultPage(a, r, c.optString("role")))
            }
        })
        add(action(a.getString(R.string.ac_name_note), "") { editSheet(c) })
        add(action(a.getString(if (blocked) R.string.ac_unblock else R.string.ac_block), a.getString(R.string.ac_block_sub)) {
            mutate("clients/update", JSONObject().put("name", name).put("disabled", !blocked)) {}
        })
        add(action(a.getString(R.string.ac_delete), a.getString(R.string.ac_delete_sub), danger = true) { deleteSheet() })
        return col
    }

    private fun editSheet(c: JSONObject) {
        var newName = c.optString("name"); var note = c.optString("note")
        val body = a.vbox()
        body.addView(a.field(a.getString(R.string.ac_name), newName, bg = T.p.high) { newName = it }.first, lp(MATCH, WRAP))
        body.addView(a.field(a.getString(R.string.ac_note), note, bg = T.p.high) { note = it }.first, lp(MATCH, WRAP).apply { topMargin = dp(8) })
        body.addView(a.button(a.getString(R.string.ac_save)) {
            val n = newName.trim()
            if (n.isEmpty()) { a.toast(a.getString(R.string.err_empty)); return@button }
            val req = JSONObject().put("name", name).put("note", note.trim())
            if (n != name) req.put("new_name", n)
            mutate("clients/update", req) { a.closeSheet(); if (n != name) a.pop() }
        }, lp(MATCH, WRAP).apply { topMargin = dp(16) })
        a.openSheet(a.getString(R.string.ac_name_note), body)
    }

    private fun deleteSheet() {
        val body = a.vbox()
        body.addView(a.text(a.getString(R.string.ac_delete_ask, name), 14f, 500, T.p.muted).also { it.setPadding(0, 0, 0, dp(14)) })
        body.addView(a.button(a.getString(R.string.ac_delete)) {
            mutate("clients/delete", JSONObject().put("name", name)) { a.closeSheet(); a.pop(); a.toast(a.getString(R.string.ac_deleted)) }
        })
        a.openSheet(a.getString(R.string.ac_delete), body)
    }
}
