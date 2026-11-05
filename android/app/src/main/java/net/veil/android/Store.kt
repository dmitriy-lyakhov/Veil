package net.veil.android

/** Состояние процесса: последняя ошибка запуска сервиса (код E_* либо текст) и время подъёма туннеля. */
object Store {
    @Volatile var lastError: String? = null
    /** SystemClock.elapsedRealtime() в момент подъёма туннеля; 0 — туннеля нет. Для таймера сессии на кольце. */
    @Volatile var since: Long = 0L
}
