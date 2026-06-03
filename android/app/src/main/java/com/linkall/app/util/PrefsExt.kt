package com.linkall.app.controller

import android.content.Context
import com.linkall.app.util.Prefs

object PrefsHolder {
    @Volatile private var appContext: Context? = null
    fun init(ctx: Context) {
        appContext = ctx.applicationContext
        // 触发 Prefs 单例
        Prefs.get(ctx)
    }
    fun get(): Prefs {
        val c = appContext ?: error("Prefs not initialised; call PrefsHolder.init in Application")
        return Prefs.get(c)
    }
}
