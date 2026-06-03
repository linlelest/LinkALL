package com.linkall.app.util

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * 加密 KV 持久化（封装 EncryptedSharedPreferences + KeyStore 派生 master key）。
 * - Master Key: AES256_GCM via AndroidKeyStore
 * - 加密: AES256_SIV (key) + AES256_GCM (value)
 * 用于存储：user token / device token / device password / server URL 等敏感数据。
 */
class Prefs private constructor(ctx: Context) {

    private val encrypted: Boolean
    private val sp: SharedPreferences = try {
        val key = MasterKey.Builder(ctx)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        EncryptedSharedPreferences.create(
            ctx,
            "linkall_secure",
            key,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
        ).also { encrypted = true }
    } catch (e: Throwable) {
        Log.e(TAG, "EncryptedSharedPreferences unavailable — storing secrets in plaintext!", e)
        encrypted = false
        ctx.getSharedPreferences("linkall", Context.MODE_PRIVATE)
    }

    private fun requireEncrypted() {
        check(encrypted) { "Secure storage unavailable. Cannot persist sensitive data." }
    }

    var token: String?
        get() = sp.getString(K_TOKEN, null)
        set(v) {
            if (v != null) requireEncrypted()
            sp.edit().putString(K_TOKEN, v).apply()
        }

    var username: String?
        get() = sp.getString(K_USERNAME, null)
        set(v) { sp.edit().putString(K_USERNAME, v).apply() }

    var serverOverride: String?
        get() = sp.getString(K_SERVER, null)
        set(v) { sp.edit().putString(K_SERVER, v).apply() }

    var deviceCode: String?
        get() = sp.getString(K_DEV_CODE, null)
        set(v) { sp.edit().putString(K_DEV_CODE, v).apply() }

    var deviceToken: String?
        get() = sp.getString(K_DEV_TOKEN, null)
        set(v) {
            if (v != null) requireEncrypted()
            sp.edit().putString(K_DEV_TOKEN, v).apply()
        }

    var devicePassword: String?
        get() = sp.getString(K_DEV_PW, null)
        set(v) {
            if (v != null) requireEncrypted()
            sp.edit().putString(K_DEV_PW, v).apply()
        }

    var locale: String
        get() = sp.getString(K_LOCALE, "zh-CN") ?: "zh-CN"
        set(v) { sp.edit().putString(K_LOCALE, v).apply() }

    // 文件断点续传进度
    var fileOffset: Long
        get() = sp.getLong(K_FILE_OFFSET, 0L)
        set(v) { sp.edit().putLong(K_FILE_OFFSET, v).apply() }

    var fileSha256: String?
        get() = sp.getString(K_FILE_SHA, null)
        set(v) { sp.edit().putString(K_FILE_SHA, v).apply() }

    /** 截屏服务是否在跑（HostedRtcSession.stop 时清） */
    var screenCapturing: Boolean
        get() = sp.getBoolean(K_SCREEN_CAPTURING, false)
        set(v) { sp.edit().putBoolean(K_SCREEN_CAPTURING, v).apply() }

    fun serverBase(): String = (serverOverride ?: "http://127.0.0.1:8080").trimEnd('/')

    fun setServerBase(url: String) {
        serverOverride = url.trimEnd('/')
    }

    fun clearAuth() {
        sp.edit()
            .remove(K_TOKEN)
            .remove(K_USERNAME)
            .apply()
    }

    fun clearDevice() {
        sp.edit()
            .remove(K_DEV_CODE)
            .remove(K_DEV_TOKEN)
            .remove(K_DEV_PW)
            .apply()
    }

    companion object {
        private const val TAG = "LinkALL/Prefs"
        const val K_TOKEN = "user.token"
        const val K_USERNAME = "user.username"
        const val K_SERVER = "server.url"
        const val K_DEV_CODE = "device.code"
        const val K_DEV_TOKEN = "device.token"
        const val K_DEV_PW = "device.password"
        const val K_LOCALE = "user.locale"
        const val K_FILE_OFFSET = "file.offset"
        const val K_FILE_SHA = "file.sha"
        const val K_SCREEN_CAPTURING = "screen.capturing"

        @Volatile private var INSTANCE: Prefs? = null
        fun get(ctx: Context): Prefs {
            return INSTANCE ?: synchronized(this) {
                INSTANCE ?: Prefs(ctx.applicationContext).also { INSTANCE = it }
            }
        }
    }
}
