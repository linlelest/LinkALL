package com.linkall.app.hosted

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.graphics.PixelFormat
import android.hardware.display.DisplayManager
import android.hardware.display.VirtualDisplay
import android.media.Image
import android.media.ImageReader
import android.media.projection.MediaProjection
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.IBinder
import android.os.SystemClock
import android.util.DisplayMetrics
import android.util.Log
import android.view.Surface
import android.view.WindowManager
import com.linkall.app.R
import com.linkall.app.ui.MainActivity
import java.nio.ByteBuffer
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

/**
 * 截屏前台服务：通过 MediaProjection 抓取屏幕 → 转 I420 → 推 WebRtcHost。
 *
 * 关键点：
 *   - **MediaProjection.Callback**：当系统主动 revoke token（Android 14+ 资源紧张、用户从快速设置撤销）
 *     时回调 onStop，需要停止推流并把状态写回 Prefs。
 *   - **rotation**：按 display.rotation 算出 0/90/180/270，传入 WebRtcHost.addFrame（WebRTC 标准）。
 *   - **frame dropping**：用 `processing` AtomicBoolean 防止重入；用 `lastFrameNanos` 控制最小帧间隔
 *     （目标 fps 30 时至少 33ms 一帧）。这样编码慢时不会堆积内存。
 *   - **stop() 公开** 暴露给 HostedRtcSession.stop() 调，释放所有资源。
 */
class ScreenCaptureService : Service() {
    private var projection: MediaProjection? = null
    private var virtualDisplay: VirtualDisplay? = null
    private var imageReader: ImageReader? = null
    private var projectionData: Intent? = null
    private var display: android.view.Display? = null
    private var width: Int = 0
    private var height: Int = 0
    private var density: Int = 1
    private val frameListeners = mutableListOf<(Image) -> Unit>()
    private val processing = AtomicBoolean(false)
    private val lastFrameNanos = AtomicLong(0L)
    /** 来自 Prefs 的目标 fps（默认 30） */
    private var targetFps: Int = 30

    private val projectionCallback = object : MediaProjection.Callback() {
        override fun onStop() {
            Log.w(TAG, "MediaProjection revoked by system; stopping capture")
            stopCapture(notifyState = true)
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        startInForeground()
        val data: Intent? = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            intent?.getParcelableExtra(EXTRA_PROJECTION_DATA, Intent::class.java)
        } else {
            @Suppress("DEPRECATION")
            intent?.getParcelableExtra(EXTRA_PROJECTION_DATA)
        }
        targetFps = (com.linkall.app.controller.PrefsHolder.get().let { 30 }.coerceIn(5, 60))
        if (data == null) {
            Log.w(TAG, "missing projection data; service idle until next start with data")
            return START_STICKY
        }
        projectionData = data
        try {
            startProjection(data)
            try { com.linkall.app.controller.PrefsHolder.get().screenCapturing = true } catch (_: Throwable) {}
        } catch (e: Throwable) {
            Log.e(TAG, "startProjection failed: ${e.message}", e)
        }
        return START_STICKY
    }

    private fun startProjection(data: Intent) {
        val mpm = getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        val p = mpm.getMediaProjection(ActivityResultOk, data)
        // 关键：注册 callback。Android 14+ 会在 token revoke 时调 onStop
        p.registerCallback(projectionCallback, null)
        projection = p
        val wm = getSystemService(Context.WINDOW_SERVICE) as WindowManager
        display = wm.defaultDisplay
        val metrics = DisplayMetrics()
        @Suppress("DEPRECATION")
        display?.getRealMetrics(metrics)
        width = metrics.widthPixels
        height = metrics.heightPixels
        density = metrics.densityDpi
        Log.i(TAG, "Projection: ${width}x${height} dpi=$density rot=${display?.rotation}")

        imageReader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, 2)
        imageReader?.setOnImageAvailableListener({ reader ->
            // 帧率节流：< 1000/targetFps 毫秒就丢
            val now = SystemClock.elapsedRealtimeNanos()
            val minIntervalNanos = 1_000_000_000L / targetFps
            if (now - lastFrameNanos.get() < minIntervalNanos) return@setOnImageAvailableListener
            // 防止重入：上一帧还没处理完就丢
            if (!processing.compareAndSet(false, true)) return@setOnImageAvailableListener
            val img = try { reader.acquireLatestImage() } catch (_: Throwable) { null }
            if (img == null) {
                processing.set(false)
                return@setOnImageAvailableListener
            }
            try {
                handleImage(img)
                lastFrameNanos.set(SystemClock.elapsedRealtimeNanos())
            } catch (t: Throwable) {
                Log.e(TAG, "handleImage: ${t.message}")
            } finally {
                try { img.close() } catch (_: Throwable) {}
                processing.set(false)
            }
        }, null)
        virtualDisplay = p.createVirtualDisplay(
            "LinkALL-Capture",
            width, height, density,
            DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
            imageReader!!.surface, null, null
        )
    }

    private fun handleImage(img: Image) {
        val plane = img.planes[0]
        val buf: ByteBuffer = plane.buffer
        val rowStride = plane.rowStride
        val pixelStride = plane.pixelStride
        val w = img.width
        val h = img.height
        val raw = ByteArray(buf.remaining())
        buf.get(raw)
        val i420 = rgbaToI420(raw, w, h, rowStride, pixelStride)
        // 把 display.rotation 转成 WebRTC I420 标准的 0/90/180/270
        val rot = when (display?.rotation) {
            Surface.ROTATION_0 -> 0
            Surface.ROTATION_90 -> 90
            Surface.ROTATION_180 -> 180
            Surface.ROTATION_270 -> 270
            else -> 0
        }
        WebRtcHost.addFrame(w, h, rot, ByteBuffer.wrap(i420))
        synchronized(frameListeners) {
            for (l in frameListeners) {
                try { l(img) } catch (_: Throwable) {}
            }
        }
    }

    private fun rgbaToI420(rgba: ByteArray, w: Int, h: Int, rowStride: Int, pixelStride: Int): ByteArray {
        val out = ByteArray(w * h + w * h / 2)
        var yi = 0
        var ui = w * h
        var vi = w * h + w * h / 4
        for (j in 0 until h) {
            val rowStart = j * rowStride
            for (i in 0 until w) {
                val off = rowStart + i * pixelStride
                val r = rgba[off].toInt() and 0xff
                val g = rgba[off + 1].toInt() and 0xff
                val b = rgba[off + 2].toInt() and 0xff
                val y = (0.257 * r + 0.504 * g + 0.098 * b + 16).toInt().coerceIn(0, 255)
                out[yi++] = y.toByte()
                if ((i and 1) == 0 && (j and 1) == 0) {
                    val u = (-0.148 * r - 0.291 * g + 0.439 * b + 128).toInt().coerceIn(0, 255)
                    val v = (0.439 * r - 0.368 * g - 0.071 * b + 128).toInt().coerceIn(0, 255)
                    out[ui++] = u.toByte()
                    out[vi++] = v.toByte()
                }
            }
        }
        return out
    }

    fun addFrameListener(l: (Image) -> Unit) {
        synchronized(frameListeners) { frameListeners.add(l) }
    }
    fun removeFrameListener(l: (Image) -> Unit) {
        synchronized(frameListeners) { frameListeners.remove(l) }
    }

    /**
     * 公开停止接口。HostedRtcSession.stop() 调。
     * @param notifyState true 时把"已停止"状态写回 Prefs
     */
    fun stopCapture(notifyState: Boolean = true) {
        try { virtualDisplay?.release() } catch (_: Throwable) {}
        try { imageReader?.close() } catch (_: Throwable) {}
        try { projection?.unregisterCallback(projectionCallback) } catch (_: Throwable) {}
        try { projection?.stop() } catch (_: Throwable) {}
        virtualDisplay = null
        imageReader = null
        projection = null
        if (notifyState) {
            try { com.linkall.app.controller.PrefsHolder.get().screenCapturing = false } catch (_: Throwable) {}
        }
        stopSelf()
    }

    private fun startInForeground() {
        val channelId = "linkall.hosted"
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val ch = NotificationChannel(channelId, getString(R.string.notify_channel_hosted), NotificationManager.IMPORTANCE_LOW)
            nm.createNotificationChannel(ch)
        }
        val pi = PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        val n: Notification = Notification.Builder(this, channelId)
            .setContentTitle(getString(R.string.notify_hosted_title))
            .setContentText(getString(R.string.notify_hosted_text, "—"))
            .setSmallIcon(android.R.drawable.stat_sys_upload)
            .setContentIntent(pi)
            .setOngoing(true)
            .build()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(NOTIFY_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PROJECTION)
        } else {
            startForeground(NOTIFY_ID, n)
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        stopCapture(notifyState = false)
    }

    companion object {
        const val TAG = "LinkALL/ScreenCap"
        const val EXTRA_PROJECTION_DATA = "extra.projection.data"
        const val NOTIFY_ID = 1001
        const val ActivityResultOk = -1 // Activity.RESULT_OK = -1
        fun makeIntent(ctx: Context, data: Intent): Intent =
            Intent(ctx, ScreenCaptureService::class.java)
                .putExtra(EXTRA_PROJECTION_DATA, data)
    }
}
