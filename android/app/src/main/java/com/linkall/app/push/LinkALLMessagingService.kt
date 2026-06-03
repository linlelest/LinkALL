package com.linkall.app.push

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import com.linkall.app.ui.MainActivity
import com.linkall.app.R
import com.linkall.app.controller.PrefsHolder
import com.linkall.app.hosted.HostedServiceController
import com.linkall.app.util.AppLog
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.util.concurrent.TimeUnit

/**
 * FCM 消息接收服务
 *
 * - onNewToken: 主动注册 token 到服务端 /api/devices/fcm-token
 * - onMessageReceived: 收到 data.action=wake_up 拉起 HostedService
 *
 * 注意：FCM 类仍编译，没有 google-services.json 时运行期 try/catch 走 fallback
 */
class LinkALLMessagingService : FirebaseMessagingService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val client by lazy {
        OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(15, TimeUnit.SECONDS)
            .build()
    }

    override fun onNewToken(token: String) {
        super.onNewToken(token)
        AppLog.i("FCM", "onNewToken: ${token.take(12)}...")
        scope.launch { registerTokenToServer(token) }
    }

    override fun onMessageReceived(message: RemoteMessage) {
        val data = message.data
        val action = data["action"]
        val title = data["title"] ?: message.notification?.title ?: "LinkALL"
        val body = data["body"] ?: message.notification?.body ?: "收到新消息"
        AppLog.i("FCM", "onMessage action=$action title=$title")
        when (action) {
            "wake_up" -> {
                // 拉起 HostedService
                try {
                    HostedServiceController.startIfNotRunning(applicationContext)
                    AppLog.i("FCM", "HostService started from FCM")
                } catch (t: Throwable) {
                    AppLog.e("FCM", "startIfNotRunning: ${t.message}")
                }
                // 同时给个系统通知，方便用户知道
                showWakeUpNotification(title, body, data)
            }
            "announcement" -> {
                showAnnouncementNotification(title, body, data)
            }
            else -> {
                // 普通推送：直接显示通知
                showWakeUpNotification(title, body, data)
            }
        }
    }

    override fun onDeletedMessages() {
        super.onDeletedMessages()
        AppLog.w("FCM", "onDeletedMessages")
    }

    private suspend fun registerTokenToServer(token: String) {
        val ctx = applicationContext
        val deviceCode = PrefsHolder.get().deviceCode ?: run {
            AppLog.w("FCM", "no deviceCode; skip register")
            return
        }
        if (deviceCode.isEmpty()) return
        val appVer = try { ctx.packageManager.getPackageInfo(ctx.packageName, 0).versionName ?: "unknown" } catch (_: Throwable) { "unknown" }
        val base = PrefsHolder.get().serverBase()
        val url = "$base/api/devices/fcm-token"
        val body = JSONObject()
            .put("device_code", deviceCode)
            .put("token", token)
            .put("app_version", appVer)
            .toString()
        try {
            val req = Request.Builder()
                .url(url)
                .post(body.toRequestBody("application/json; charset=utf-8".toMediaType()))
                .build()
            client.newCall(req).execute().use { r ->
                if (!r.isSuccessful) {
                    AppLog.w("FCM", "register: ${r.code} ${r.message}")
                } else {
                    AppLog.i("FCM", "token registered to server")
                }
            }
        } catch (t: Throwable) {
            AppLog.e("FCM", "register: ${t.message}")
        }
    }

    private fun showWakeUpNotification(title: String, body: String, data: Map<String, String>) {
        val intent = Intent(this, MainActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        }
        val pending = PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        val n = NotificationCompat.Builder(this, CHANNEL_WAKE)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle(title)
            .setContentText(body)
            .setAutoCancel(true)
            .setContentIntent(pending)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .build()
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        ensureChannel(CHANNEL_WAKE, "远程唤醒", NotificationManager.IMPORTANCE_HIGH)
        nm.notify(NOTIFY_WAKE, n)
    }

    private fun showAnnouncementNotification(title: String, body: String, data: Map<String, String>) {
        val intent = Intent(this, MainActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        }
        val pending = PendingIntent.getActivity(
            this, 1, intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        val n = NotificationCompat.Builder(this, CHANNEL_ANNOUNCE)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle(title)
            .setContentText(body)
            .setAutoCancel(true)
            .setContentIntent(pending)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .build()
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        ensureChannel(CHANNEL_ANNOUNCE, "系统公告", NotificationManager.IMPORTANCE_DEFAULT)
        nm.notify(NOTIFY_ANNOUNCE, n)
    }

    private fun ensureChannel(id: String, name: String, importance: Int) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (nm.getNotificationChannel(id) != null) return
        val ch = NotificationChannel(id, name, importance)
        nm.createNotificationChannel(ch)
    }

    companion object {
        const val CHANNEL_WAKE = "linkall_wake"
        const val CHANNEL_ANNOUNCE = "linkall_announce"
        const val NOTIFY_WAKE = 1001
        const val NOTIFY_ANNOUNCE = 1002
    }
}
