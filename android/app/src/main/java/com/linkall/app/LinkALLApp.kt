package com.linkall.app

import android.app.Application
import com.linkall.app.controller.PrefsHolder
import com.linkall.app.di.appModule
import com.linkall.app.util.AppLog
import com.linkall.app.util.Prefs
import org.koin.android.ext.koin.androidContext
import org.koin.android.ext.koin.androidLogger
import org.koin.core.context.startKoin
import org.koin.core.logger.Level

class LinkALLApp : Application() {
    override fun onCreate() {
        super.onCreate()
        PrefsHolder.init(this)
        AppLog.init(this)
        startKoin {
            androidLogger(Level.INFO)
            androidContext(this@LinkALLApp)
            modules(appModule)
        }
    }
}
