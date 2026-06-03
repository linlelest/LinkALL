package com.linkall.app.ui.admin

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import kotlinx.coroutines.launch
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.linkall.app.R
import com.linkall.app.data.api.AdminStats
import com.linkall.app.data.api.Announcement
import com.linkall.app.data.api.OtaInfo
import com.linkall.app.data.repo.AdminRepo
import com.linkall.app.ui.components.Card
import com.linkall.app.ui.components.MonoText
import com.linkall.app.ui.components.MutedText
import com.linkall.app.ui.components.PrimaryButton
import org.koin.compose.koinInject

@Composable
fun AdminScreen() {
    val repo: AdminRepo = koinInject()
    var stats by remember { mutableStateOf<AdminStats?>(null) }
    var ota by remember { mutableStateOf<OtaInfo?>(null) }
    var latest by remember { mutableStateOf<Announcement?>(null) }
    var err by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(Unit) {
        runCatching {
            stats = repo.stats()
            ota = repo.ota()
            latest = repo.announcements().firstOrNull()
        }.onFailure { err = it.message }
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(12.dp).verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Text(stringResource(R.string.admin_title), style = MaterialTheme.typography.titleLarge)
        if (err != null) Text(err!!, color = MaterialTheme.colorScheme.error)

        Card {
            Text(stringResource(R.string.admin_account), color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(4.dp))
            MonoText("用户 / 设备 / 流量：可在服务端 `/api/admin/*`")
        }

        Card {
            Text(stringResource(R.string.admin_announcements), color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(4.dp))
            if (latest != null) {
                MonoText("• " + latest!!.title)
                MutedText(latest!!.contentMd.take(80))
            } else MutedText("（暂无）")
        }

        Card {
            Text(stringResource(R.string.admin_ota), color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(4.dp))
            if (ota != null) {
                MonoText("has_update=${ota!!.has_update} version=${ota!!.version} force=${ota!!.forceUpdate}")
            } else MutedText("（未拉取）")
        }

        Card {
            Text("Server", color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(4.dp))
            if (stats != null) {
                MonoText("users=${stats!!.users} devices=${stats!!.devices} online=${stats!!.online}")
                MonoText("sessions=${stats!!.sessions} tx=${stats!!.bytesTx} rx=${stats!!.bytesRx}")
            } else MutedText("（未拉取，需要 admin 权限）")
        }

        PrimaryButton("刷新", modifier = Modifier.fillMaxWidth()) {
            scope.launch {
                runCatching {
                    stats = repo.stats()
                    ota = repo.ota()
                    latest = repo.announcements().firstOrNull()
                }.onFailure { err = it.message }
            }
        }
    }
}
