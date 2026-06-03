package com.linkall.app.ui.hosted

import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.linkall.app.R
import com.linkall.app.data.repo.AuthRepo
import com.linkall.app.data.repo.DeviceRepo
import com.linkall.app.hosted.HostedServiceController
import com.linkall.app.ui.components.Card
import com.linkall.app.ui.components.GhostButton
import com.linkall.app.ui.components.LabeledSwitch
import com.linkall.app.ui.components.MonoText
import com.linkall.app.ui.components.MutedText
import com.linkall.app.ui.components.PrimaryButton
import com.linkall.app.util.Crypto
import com.linkall.app.util.Prefs
import kotlinx.coroutines.launch
import org.koin.compose.koinInject

@Composable
fun HostedScreen(locale: String, onLocaleChange: (String) -> Unit) {
    val prefs: Prefs = koinInject()
    val ctx = LocalContext.current
    val deviceRepo: DeviceRepo = koinInject()
    val authRepo: AuthRepo = koinInject()
    val hosted: HostedServiceController = koinInject()
    val scope = rememberCoroutineScope()

    var deviceCode by remember { mutableStateOf(prefs.deviceCode ?: "") }
    var devicePassword by remember { mutableStateOf(prefs.devicePassword ?: "") }
    var deviceName by remember { mutableStateOf("") }
    var allowAnon by remember { mutableStateOf(true) }
    var requireCode by remember { mutableStateOf(true) }
    var accept by remember { mutableStateOf(true) }
    var loggedIn by remember { mutableStateOf(!prefs.deviceCode.isNullOrEmpty()) }
    var serviceOn by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    var info by remember { mutableStateOf<String?>(null) }

    fun refresh() {
        loggedIn = !prefs.deviceCode.isNullOrEmpty()
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(16.dp)
            .verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(stringResource(R.string.hosted_title), style = MaterialTheme.typography.titleLarge)
        MutedText(stringResource(R.string.hosted_status) + ": " + (if (serviceOn) stringResource(R.string.hosted_status_running) else if (loggedIn) stringResource(R.string.hosted_status_paused) else stringResource(R.string.hosted_status_offline)))

        // 登录/注册卡片
        if (!loggedIn) {
            Card {
                OutlinedTextField(value = deviceCode, onValueChange = { deviceCode = it.uppercase() }, label = { Text(stringResource(R.string.hosted_device_code)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(value = devicePassword, onValueChange = { devicePassword = it }, label = { Text(stringResource(R.string.hosted_device_password)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(value = deviceName, onValueChange = { deviceName = it }, label = { Text(stringResource(R.string.hosted_device_name)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                if (err != null) Text(err!!, color = MaterialTheme.colorScheme.error)
                if (info != null) Text(info!!, color = MaterialTheme.colorScheme.primary)
                Spacer(Modifier.height(8.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    PrimaryButton(text = "注册", modifier = Modifier.weight(1f)) {
                        err = null; info = null
                        if (deviceCode.isBlank()) deviceCode = Crypto.randomCode12()
                        if (devicePassword.length < 6) { err = "设备码至少 6 位"; return@PrimaryButton }
                        scope.launch {
                            runCatching { deviceRepo.register(deviceCode, devicePassword, deviceName.ifBlank { null }) }
                                .onSuccess { d -> allowAnon = d.allowAnonymous; requireCode = d.requireDeviceCode; accept = d.acceptConnections; refresh() }
                                .onFailure { err = it.message }
                        }
                    }
                    GhostButton(text = "登录", modifier = Modifier.weight(1f)) {
                        err = null
                        scope.launch {
                            runCatching { deviceRepo.login(deviceCode, devicePassword) }
                                .onSuccess { d -> allowAnon = d.allowAnonymous; requireCode = d.requireDeviceCode; accept = d.acceptConnections; refresh() }
                                .onFailure { err = it.message }
                        }
                    }
                }
            }
        } else {
            // 已登录：显示状态/安全设置
            Card {
                Text(stringResource(R.string.hosted_device_code), color = MaterialTheme.colorScheme.onSurfaceVariant)
                MonoText(prefs.deviceCode ?: "")
                Spacer(Modifier.height(4.dp))
                MutedText("Token 已保存于本机；权限开关可即时同步到服务端。")
                Spacer(Modifier.height(8.dp))
                LabeledSwitch(stringResource(R.string.hosted_allow_anon), allowAnon) { v ->
                    allowAnon = v
                    scope.launch {
                        runCatching { deviceRepo.updateFlags(0L, v, requireCode, accept) }
                    }
                }
                LabeledSwitch(stringResource(R.string.hosted_require_code), requireCode) { v ->
                    requireCode = v
                    scope.launch { runCatching { deviceRepo.updateFlags(0L, allowAnon, v, accept) } }
                }
                LabeledSwitch(stringResource(R.string.hosted_accept), accept) { v ->
                    accept = v
                    scope.launch { runCatching { deviceRepo.updateFlags(0L, allowAnon, requireCode, v) } }
                }
                Spacer(Modifier.height(8.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    if (serviceOn) {
                        GhostButton(stringResource(R.string.hosted_stop_foreground), modifier = Modifier.weight(1f)) {
                            hosted.stop(ctx)
                            serviceOn = false
                        }
                    } else {
                        PrimaryButton(stringResource(R.string.hosted_foreground), modifier = Modifier.weight(1f)) {
                            hosted.start(ctx)
                            serviceOn = true
                        }
                    }
                    GhostButton(stringResource(R.string.hosted_quit), modifier = Modifier.weight(1f)) {
                        hosted.stop(ctx)
                        serviceOn = false
                        deviceRepo.logout()
                        refresh()
                    }
                }
                Spacer(Modifier.height(8.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    GhostButton(stringResource(R.string.hosted_open_settings), modifier = Modifier.weight(1f)) {
                        ctx.startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                    }
                    GhostButton(stringResource(R.string.hosted_capture), modifier = Modifier.weight(1f)) {
                        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                            val intent = Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION, Uri.parse("package:" + ctx.packageName))
                            intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                            ctx.startActivity(intent)
                        }
                    }
                }
                Spacer(Modifier.height(8.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    GhostButton(stringResource(R.string.hosted_battery), modifier = Modifier.weight(1f)) {
                        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                            val intent = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, Uri.parse("package:" + ctx.packageName))
                            intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                            try { ctx.startActivity(intent) } catch (_: Throwable) { ctx.startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
                        }
                    }
                    GhostButton(stringResource(R.string.hosted_boot), modifier = Modifier.weight(1f)) {
                        val brands = listOf(
                            "com.huawei.systemmanager" to "com.huawei.systemmanager.startupmgr.ui.StartupNormalAppListActivity",
                            "com.miui.securitycenter" to "com.miui.permcenter.autostart.AutoStartManagementActivity",
                            "com.oppo.safe" to "com.coloros.safecenter.permission.startup.StartupAppListActivity",
                            "com.vivo.permissionmanager" to "com.vivo.permissionmanager.activity.BgStartUpManagerActivity"
                        )
                        var ok = false
                        for ((pkg, cls) in brands) {
                            try {
                                val i = Intent().setComponent(android.content.ComponentName(pkg, cls)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                                ctx.startActivity(i)
                                ok = true
                                break
                            } catch (_: Throwable) {}
                        }
                        if (!ok) ctx.startActivity(Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.parse("package:" + ctx.packageName)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                    }
                }
            }
        }

        // 语言切换
        Card {
            Text(stringResource(R.string.admin_locale), color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(4.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                GhostButton("中文", modifier = Modifier.weight(1f)) { onLocaleChange("zh-CN") }
                GhostButton("English", modifier = Modifier.weight(1f)) { onLocaleChange("en-US") }
            }
        }
    }
}
