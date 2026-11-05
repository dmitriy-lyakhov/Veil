package net.veil.android

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.security.KeyStore
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** AES-256-GCM, ключ в Android Keystore (без привязки к биометрии). Формат файла: [12 байт nonce][шифртекст+тег]. */
object SecureStore {
    private const val ALIAS = "veil_master_v1"

    private fun key(): SecretKey {
        val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (ks.getKey(ALIAS, null) as? SecretKey)?.let { return it }
        val g = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        g.init(
            KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build()
        )
        return g.generateKey()
    }

    fun encrypt(plain: ByteArray): ByteArray {
        val c = Cipher.getInstance("AES/GCM/NoPadding")
        c.init(Cipher.ENCRYPT_MODE, key())
        return c.iv + c.doFinal(plain)
    }

    fun decrypt(blob: ByteArray): ByteArray {
        val c = Cipher.getInstance("AES/GCM/NoPadding")
        c.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, blob, 0, 12))
        return c.doFinal(blob, 12, blob.size - 12)
    }
}

class Profile(val id: String, var name: String, val protocol: String, val link: String, val added: Long)

/** Профили: зашифрованный файл profiles.bin; активный профиль — в настройках. Ссылки в журнал не пишем. */
object Profiles {
    private var cache: MutableList<Profile>? = null

    private fun file(c: Context) = File(c.filesDir, "profiles.bin")

    @Synchronized
    fun all(c: Context): List<Profile> {
        cache?.let { return it }
        val out = mutableListOf<Profile>()
        val f = file(c)
        if (f.exists()) try {
            val arr = JSONArray(String(SecureStore.decrypt(f.readBytes()), Charsets.UTF_8))
            for (i in 0 until arr.length()) {
                val o = arr.getJSONObject(i)
                out += Profile(o.getString("id"), o.getString("name"), o.optString("protocol", "veil"), o.getString("link"), o.optLong("added"))
            }
        } catch (_: Exception) { /* повреждённый файл: начинаем с пустого списка */ }
        cache = out
        migrateLegacy(c, out)
        return out
    }

    private fun migrateLegacy(c: Context, out: MutableList<Profile>) {
        val sp = c.getSharedPreferences("veil", Context.MODE_PRIVATE)
        val old = sp.getString("link", "") ?: ""
        if (old.isNotBlank()) {
            try { addRaw(c, old, null) } catch (_: Exception) { }
            sp.edit().remove("link").apply()
        }
    }

    @Synchronized
    private fun save(c: Context) {
        val arr = JSONArray()
        for (p in cache ?: return) {
            arr.put(JSONObject().put("id", p.id).put("name", p.name).put("protocol", p.protocol).put("link", p.link).put("added", p.added))
        }
        val tmp = File(c.filesDir, "profiles.tmp")
        tmp.writeBytes(SecureStore.encrypt(arr.toString().toByteArray(Charsets.UTF_8)))
        tmp.renameTo(file(c))
    }

    /** Импорт текста (ссылка veil://, .conf WireGuard/AmneziaWG, JSON). Бросает Exception с кодом E_* или текстом. */
    @Synchronized
    fun addRaw(c: Context, text: String, name: String?): Profile {
        val j = JSONObject(veilcore.Veilcore.import_(text, name ?: ""))
        val list = all(c) as MutableList<Profile>
        val p = Profile(java.util.UUID.randomUUID().toString(), j.getString("name"), j.getString("protocol"), j.getString("link"), System.currentTimeMillis())
        list += p
        save(c)
        if (Settings.activeId(c) == null) Settings.setActive(c, p.id)
        return p
    }

    @Synchronized
    fun rename(c: Context, id: String, name: String) {
        val p = all(c).firstOrNull { it.id == id } ?: return
        val link = veilcore.Veilcore.rename(p.link, name)
        val list = all(c) as MutableList<Profile>
        val i = list.indexOf(p)
        list[i] = Profile(p.id, name.trim(), p.protocol, link, p.added)
        save(c)
    }

    @Synchronized
    fun delete(c: Context, id: String) {
        val list = all(c) as MutableList<Profile>
        list.removeAll { it.id == id }
        save(c)
        if (Settings.activeId(c) == id) Settings.setActive(c, list.firstOrNull()?.id)
    }

    fun active(c: Context): Profile? {
        val l = all(c)
        return l.firstOrNull { it.id == Settings.activeId(c) } ?: l.firstOrNull()
    }

    /** Резервная копия: JSON со всеми профилями (открытым текстом — файл хранит пользователь). */
    fun exportJson(c: Context): String {
        val arr = JSONArray()
        for (p in all(c)) arr.put(JSONObject().put("name", p.name).put("link", p.link))
        return JSONObject().put("veil_backup", 1).put("profiles", arr).toString(2)
    }

    fun importBackup(c: Context, text: String): Int {
        val arr = JSONObject(text).getJSONArray("profiles")
        var n = 0
        for (i in 0 until arr.length()) {
            val o = arr.getJSONObject(i)
            try { addRaw(c, o.getString("link"), o.optString("name")); n++ } catch (_: Exception) { }
        }
        return n
    }
}

/** Настройки приложения (не секретные). */
object Settings {
    private fun sp(c: Context) = c.getSharedPreferences("settings", Context.MODE_PRIVATE)

    fun theme(c: Context) = sp(c).getString("theme", "system")!!
    fun lang(c: Context) = sp(c).getString("lang", "system")!!
    fun set(c: Context, k: String, v: String) = sp(c).edit().putString(k, v).apply()
    fun bool(c: Context, k: String, d: Boolean = false) = sp(c).getBoolean(k, d)
    fun setBool(c: Context, k: String, v: Boolean) = sp(c).edit().putBoolean(k, v).apply()
    fun str(c: Context, k: String, d: String = "") = sp(c).getString(k, d) ?: d

    fun activeId(c: Context): String? = sp(c).getString("active", null)
    fun setActive(c: Context, id: String?) = sp(c).edit().putString("active", id).apply()

    // автозапуск
    fun autoApp(c: Context) = bool(c, "auto_app")
    fun autoBoot(c: Context) = bool(c, "auto_boot")
    /** Пользователь хочет, чтобы VPN работал (выставляется при подключении, снимается при ручном отключении). */
    fun wantOn(c: Context) = bool(c, "want_on")
    fun setWantOn(c: Context, v: Boolean) = setBool(c, "want_on", v)

    // раздельное туннелирование
    fun appsOn(c: Context) = bool(c, "split_apps_on")
    fun appsMode(c: Context) = str(c, "split_apps_mode", "via") // via | direct
    fun apps(c: Context): Set<String> = sp(c).getStringSet("split_apps", emptySet())!!.toSet()
    fun setApps(c: Context, s: Set<String>) = sp(c).edit().putStringSet("split_apps", s).apply()
    fun sitesOn(c: Context) = bool(c, "split_sites_on")
    fun sitesMode(c: Context) = str(c, "split_sites_mode", "via")
    fun sites(c: Context): List<String> = str(c, "split_sites").split("\n").map { it.trim() }.filter { it.isNotEmpty() }
    fun setSites(c: Context, l: List<String>) = set(c, "split_sites", l.joinToString("\n"))

    fun splitCount(c: Context): Int =
        (if (appsOn(c)) apps(c).size else 0) + (if (sitesOn(c)) sites(c).size else 0)
    fun splitActive(c: Context) = (appsOn(c) && apps(c).isNotEmpty()) || (sitesOn(c) && sites(c).isNotEmpty())

    fun dns(c: Context) = str(c, "dns_override")
}

/** Локальные счётчики трафика: по дням и месяцам (только на устройстве). */
object Stats {
    private fun sp(c: Context) = c.getSharedPreferences("traffic", Context.MODE_PRIVATE)
    private fun day() = SimpleDateFormat("yyyyMMdd", Locale.US).format(Date())
    private fun month() = SimpleDateFormat("yyyyMM", Locale.US).format(Date())

    @Synchronized
    fun add(c: Context, rx: Long, tx: Long) {
        if (rx <= 0 && tx <= 0) return
        val e = sp(c).edit()
        for (k in listOf("d" + day(), "m" + month())) {
            e.putLong("$k.rx", sp(c).getLong("$k.rx", 0) + maxOf(rx, 0))
            e.putLong("$k.tx", sp(c).getLong("$k.tx", 0) + maxOf(tx, 0))
        }
        e.apply()
    }

    fun today(c: Context) = Pair(sp(c).getLong("d${day()}.rx", 0), sp(c).getLong("d${day()}.tx", 0))
    fun month(c: Context) = Pair(sp(c).getLong("m${month()}.rx", 0), sp(c).getLong("m${month()}.tx", 0))
}
