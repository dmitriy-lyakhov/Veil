package net.veil.android

import android.content.Context

/** Последний профиль (ссылка) в SharedPreferences и последняя ошибка запуска сервиса. Ссылку не логируем. */
object Store {
    @Volatile var lastError: String? = null

    private const val PREFS = "veil"
    private const val KEY_LINK = "link"

    fun loadLink(c: Context): String =
        c.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_LINK, "") ?: ""

    fun saveLink(c: Context, link: String) {
        c.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putString(KEY_LINK, link).apply()
    }
}
