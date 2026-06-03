package com.linkall.app.util

import android.content.Context
import android.os.Build
import android.util.Log
import com.linkall.app.controller.PrefsHolder
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.io.File
import java.io.FileWriter
import java.io.PrintWriter
import java.io.StringWriter
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.LinkedBlockingQueue

/**
 * Android 客户端结构化日志 + 崩溃上报
 *
 * - 文件输出：`filesDir/logs/app-YYYYMMDD.log`（5MB 轮转，保留 5 个）
 * - 内存缓冲：最多 500 条，HTTP 上报到 `/api/log`
 * - 崩溃捕获：默认 Thread.UncaughtExceptionHandler + 自定义
 *
 * 调用方式：
 *   AppLog.init(applicationContext)  // 在 LinkALLApp.onCreate 调
 *   AppLog.i("WebRtc", "session open")  // 任意处
 *   AppLog.reportCrash(throwable)  // 主动上报
 */
object AppLog {
    private const val TAG = "LinkALL/AppLog"
    private val dateFormat = SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.US)
    private val dayFormat = SimpleDateFormat("yyyyMMdd", Locale.US)
    private val queue = LinkedBlockingQueue<LogEntry>()
    private const val MAX_FILE_BYTES: Long = 5L * 1024 * 1024
    private const val MAX_FILES = 5
    private const val MAX_BUFFER = 500
    @Volatile private var initialized = false
    @Volatile private var appContext: Context? = null
    @Volatile private var deviceCode: String = ""
    @Volatile private var appVersion: String = "unknown"
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val pending: ArrayDeque<LogEntry> = ArrayDeque()

    enum class Level(val str: String) { TRACE("trace"), DEBUG("debug"), INFO("info"), WARN("warn"), ERROR("error"), FATAL("fatal") }

    data class LogEntry(
        val ts: Long,
        val level: String,
        val source: String,
        val message: String,
        val extra: String? = null
    ) {
        fun toJson(): JSONObject {
            val o = JSONObject()
            o.put("ts", ts)
            o.put("level", level)
            o.put("source", source)
            o.put("message", message)
            extra?.let { o.put("extra", it) }
            return o
        }
    }

    fun init(ctx: Context) {
        if (initialized) return
        appContext = ctx.applicationContext
        deviceCode = PrefsHolder.get().deviceCode ?: ""
        appVersion = try { ctx.packageManager.getPackageInfo(ctx.packageName, 0).versionName ?: "unknown" } catch (_: Throwable) { "unknown" }
        // 注册全局崩溃钩子
        val prev = Thread.getDefaultUncaughtExceptionHandler()
        Thread.setDefaultUncaughtExceptionHandler { t, e ->
            try { reportCrashInternal(t, e) } catch (_: Throwable) {}
            prev?.uncaughtException(t, e)
        }
        initialized = true
        i("AppLog", "init device=$deviceCode ver=$appVersion sdk=${Build.VERSION.SDK_INT}")
    }

    fun t(source: String, msg: String) = log(Level.TRACE, source, msg, null)
    fun d(source: String, msg: String) = log(Level.DEBUG, source, msg, null)
    fun i(source: String, msg: String) = log(Level.INFO, source, msg, null)
    fun w(source: String, msg: String) = log(Level.WARN, source, msg, null)
    fun e(source: String, msg: String) = log(Level.ERROR, source, msg, null)
    fun f(source: String, msg: String) = log(Level.FATAL, source, msg, null)

    fun log(level: Level, source: String, msg: String, extra: String?) {
        if (!initialized) return
        val entry = LogEntry(System.currentTimeMillis(), level.str, source, msg, extra)
        val line = "${dateFormat.format(Date(entry.ts))} [${level.str.uppercase()}] $source - $msg${extra?.let { " | $it" } ?: ""}\n"
        if (level.ordinal >= Level.WARN.ordinal) {
            Log.w(TAG, line.trimEnd())
        } else {
            Log.d(TAG, line.trimEnd())
        }
        synchronized(pending) {
            pending.addLast(entry)
            if (pending.size > MAX_BUFFER) pending.removeFirst()
        }
        // 异步写文件
        scope.launch { writeToFile(line) }
    }

    /** 主动上报 throwable 到 /api/crash */
    fun reportCrash(t: Throwable) {
        try {
            reportCrashInternal(Thread.currentThread(), t)
        } catch (_: Throwable) {}
    }

    private fun reportCrashInternal(t: Thread, e: Throwable) {
        val sw = StringWriter()
        e.printStackTrace(PrintWriter(sw))
        val stack = sw.toString()
        val message = "${e.javaClass.simpleName}: ${e.message ?: ""}"
        f("UncaughtException", "$message\n$stack")
        scope.launch { uploadCrash(message, stack) }
    }

    private suspend fun writeToFile(line: String) {
        val ctx = appContext ?: return
        val dir = File(ctx.filesDir, "logs").apply { mkdirs() }
        val file = File(dir, "app-${dayFormat.format(Date())}.log")
        try {
            if (file.exists() && file.length() + line.length > MAX_FILE_BYTES) {
                rotate(dir)
            }
            FileWriter(file, true).use { it.write(line) }
        } catch (t: Throwable) {
            Log.w(TAG, "write: ${t.message}")
        }
    }

    private fun rotate(dir: File) {
        val logs = dir.listFiles { f -> f.extension == "log" } ?: return
        val sorted = logs.sortedBy { it.name }
        if (sorted.size >= MAX_FILES) {
            sorted.firstOrNull()?.delete()
        }
        // app-20260602.log -> app-20260602.1.log
        sorted.reversed().forEach { f ->
            val name = f.nameWithoutExtension
            val newName = "$name.1.log"
            f.renameTo(File(dir, newName))
        }
    }

    /** 取出待上传日志，drain from buffer */
    fun drainPending(limit: Int = 200): List<LogEntry> {
        synchronized(pending) {
            val n = limit.coerceAtMost(pending.size)
            return pending.toList().take(n).also { repeat(n) { pending.removeFirst() } }
        }
    }

    fun pendingSize(): Int = synchronized(pending) { pending.size }

    private suspend fun uploadCrash(message: String, stack: String) {
        val ctx = appContext ?: return
        try {
            val body = JSONObject()
                .put("device_code", deviceCode)
                .put("platform", "android")
                .put("app_version", appVersion)
                .put("os_version", "Android ${Build.VERSION.RELEASE} (SDK ${Build.VERSION.SDK_INT})")
                .put("level", "fatal")
                .put("source", "UncaughtException")
                .put("message", message)
                .put("stack", stack.take(16384))
            postJson("$baseUrl/api/crash", body)
        } catch (t: Throwable) {
            Log.w(TAG, "uploadCrash: ${t.message}")
        }
    }

    /** 把 buffer 里的日志批量上报 */
    suspend fun uploadPending() {
        val ctx = appContext ?: return
        val entries = drainPending(200)
        if (entries.isEmpty()) return
        try {
            val arr = org.json.JSONArray()
            for (e in entries) arr.put(e.toJson())
            val body = JSONObject()
                .put("device_code", deviceCode)
                .put("platform", "android")
                .put("app_version", appVersion)
                .put("entries", arr)
            postJson("$baseUrl/api/log", body)
        } catch (t: Throwable) {
            // 上传失败：放回 buffer
            synchronized(pending) {
                entries.reversed().forEach { pending.addFirst(it) }
            }
            Log.w(TAG, "uploadPending: ${t.message}")
        }
    }

    private val baseUrl: String
        get() = PrefsHolder.get().serverBase()

    private fun postJson(url: String, body: JSONObject) {
        val req = okhttp3.Request.Builder()
            .url(url)
            .post(okhttp3.RequestBody.create("application/json; charset=utf-8".toMediaType(), body.toString()))
            .build()
        val client = okhttp3.OkHttpClient.Builder()
            .connectTimeout(10, java.util.concurrent.TimeUnit.SECONDS)
            .readTimeout(15, java.util.concurrent.TimeUnit.SECONDS)
            .build()
        client.newCall(req).execute().use { /* ignore */ }
    }
}

// Kotlin extension to MediaType
private fun String.toMediaType(): okhttp3.MediaType = okhttp3.MediaType.parse(this)!!
