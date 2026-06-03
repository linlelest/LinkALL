package com.linkall.app.hosted

import android.content.Context
import android.content.Intent
import android.os.Build
import android.util.Log
import com.linkall.app.controller.PrefsHolder
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Hosted 服务编排：前台服务 / 信令客户端 / WebRtc 主机端。
 *
 * 完整流程：
 *   1) start() 启动持久信令 WS（hello kind=controlled）
 *   2) 收到 controller 发的 `request` → 触发 onRequestReceived 回调
 *   3) MainActivity 弹确认框；用户允许后回调 accept(ctx, projectionData)
 *   4) accept() 拉起 ScreenCaptureService（带 MediaProjection data） + 启动 HostedRtcSession
 *   5) 收到 controller 的 offer/answer/ice/cmd 走 HostedRtcSession
 */
class HostedServiceController(private val ctx: Context) {

    @Volatile private var ws: WebSocket? = null
    @Volatile private var httpBase: String = ""
    @Volatile private var wsUrl: String = ""
    private val client = OkHttpClient.Builder().pingInterval(20, TimeUnit.SECONDS).build()
    @Volatile private var currentControllerId: String? = null
    @Volatile private var requestInFlight = AtomicBoolean(false)

    /** 收到 request 时回调（MainActivity 用来弹确认框） */
    var onRequestReceived: ((controllerId: String, mode: String, requireCode: Boolean) -> Unit)? = null
    /** 错误回调 */
    var onError: ((String) -> Unit)? = null
    /** 信令状态变化回调 */
    var onStatus: ((String) -> Unit)? = null

    fun start(ctx: Context) {
        if (ws != null) return
        val prefs = PrefsHolder.get()
        val token = prefs.token ?: ""
        val deviceCode = prefs.deviceCode ?: ""
        if (token.isEmpty() || deviceCode.isEmpty()) {
            Log.w(TAG, "start: missing token or device_code")
            onError?.invoke("missing_credentials")
            return
        }
        httpBase = prefs.serverBase()
        wsUrl = httpBase.replace("http://", "ws://").replace("https://", "wss://") + "/ws/signaling"
        val req = Request.Builder().url(wsUrl).build()
        onStatus?.invoke("ws:connecting")
        ws = client.newWebSocket(req, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                onStatus?.invoke("ws:open")
                val hello = JSONObject().put("type", "hello").put("data",
                    JSONObject().put("kind", "controlled").put("device_code", deviceCode).put("token", token))
                webSocket.send(hello.toString())
            }
            override fun onMessage(webSocket: WebSocket, text: String) {
                try {
                    val env = JSONObject(text)
                    when (env.optString("type")) {
                        "request" -> {
                            if (requestInFlight.get()) return
                            val data = env.optJSONObject("data") ?: return
                            val controllerId = env.optString("from", "")
                            val mode = data.optString("mode", "anonymous")
                            val requireCode = data.optBoolean("require_device_code", false)
                            currentControllerId = controllerId
                            onRequestReceived?.invoke(controllerId, mode, requireCode)
                        }
                        "error" -> onError?.invoke(env.optString("msg", "unknown"))
                    }
                } catch (e: Throwable) {
                    Log.w(TAG, "onMessage parse: ${e.message}")
                }
            }
            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                onStatus?.invoke("ws:err ${t.message}")
                ws = null
                // 简易重连（生产应做指数退避）
                try { ctx.let { android.os.Handler(ctx.mainLooper).postDelayed({ start(ctx) }, 3000) } } catch (_: Throwable) {}
            }
            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                onStatus?.invoke("ws:closed")
                ws = null
            }
        })
    }

    /**
     * 用户同意后调用：拉起截屏前台服务 + 启动 HostedRtcSession。
     * @param projectionData MediaProjection token Intent（来自 ActivityResult 回调）
     * @param iceServers 服务端 /api/config 下发
     */
    fun accept(projectionData: Intent, iceServers: List<Pair<String, String?>>) {
        requestInFlight.set(true)
        val controllerId = currentControllerId ?: run {
            requestInFlight.set(false)
            Log.w(TAG, "accept: no controller in flight")
            return
        }
        // 1) 启动截屏服务
        val intent = ScreenCaptureService.makeIntent(ctx, projectionData)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            ctx.startForegroundService(intent)
        } else {
            ctx.startService(intent)
        }
        // 2) 启动 RTC 会话
        val prefs = PrefsHolder.get()
        HostedRtcSession.start(
            ctx = ctx,
            targetWs = wsUrl,
            token = prefs.token ?: "",
            deviceCode = prefs.deviceCode ?: "",
            controllerId = controllerId,
            iceServers = iceServers,
        )
    }

    /** 用户拒绝 request */
    fun deny() {
        // 可选：发个 NACK 消息；当前服务端 ack 是被控端推的，先不动
        currentControllerId = null
        requestInFlight.set(false)
    }

    /**
     * 主控会话结束：调 HostedRtcSession.stop() + 关截屏服务
     */
    fun stopSession() {
        try { HostedRtcSession.stop() } catch (_: Throwable) {}
        try { ctx.stopService(Intent(ctx, ScreenCaptureService::class.java)) } catch (_: Throwable) {}
        requestInFlight.set(false)
        currentControllerId = null
    }

    fun stop(ctx: Context) {
        try { ws?.close(1000, "bye") } catch (_: Throwable) {}
        ws = null
        stopSession()
        Log.i(TAG, "hosted service stop")
    }

    companion object {
        const val TAG = "LinkALL/HostedCtrl"
        @Volatile private var instanceRef: HostedServiceController? = null
        @Volatile private var appContextRef: android.content.Context? = null

        fun startIfNotRunning(ctx: android.content.Context) {
            val c = ctx.applicationContext
            appContextRef = c
            val current = instanceRef
            if (current != null) {
                android.util.Log.i(TAG, "startIfNotRunning: already running")
                return
            }
            // 启一个最简 HostedServiceController 走信令 WS（不直接接 RTC，由主控再发起 accept）
            val ctrl = HostedServiceController(c)
            instanceRef = ctrl
            ctrl.start(c)
        }

        fun stopIfRunning() {
            val c = appContextRef ?: return
            instanceRef?.stop(c)
            instanceRef = null
        }
    }
}
