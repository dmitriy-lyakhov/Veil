package net.veil.android

/** Последняя ошибка запуска сервиса (в памяти процесса); код E_* либо текст. */
object Store {
    @Volatile var lastError: String? = null
}
