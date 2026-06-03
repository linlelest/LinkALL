package com.linkall.app.ui.nav

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AdminPanelSettings
import androidx.compose.material.icons.filled.Computer
import androidx.compose.material.icons.filled.Smartphone
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import com.linkall.app.R
import com.linkall.app.ui.admin.AdminScreen
import com.linkall.app.ui.controller.ControllerScreen
import com.linkall.app.ui.hosted.HostedScreen
import com.linkall.app.ui.login.LoginScreen

enum class Tab { Login, Hosted, Controller, Admin }

@Composable
fun RootScaffold(locale: String, onLocaleChange: (String) -> Unit) {
    var tab by remember { mutableStateOf(Tab.Login) }

    Scaffold(
        bottomBar = {
            // 仅在已登录时显示
            // 简化：始终显示；点 Login 也只是空操作
            NavigationBar {
                listOf(
                    Triple(Tab.Hosted, R.string.tab_hosted, Icons.Filled.Computer),
                    Triple(Tab.Controller, R.string.tab_controller, Icons.Filled.Smartphone),
                    Triple(Tab.Admin, R.string.tab_admin, Icons.Filled.AdminPanelSettings)
                ).forEach { (t, label, icon) ->
                    NavigationBarItem(
                        selected = tab == t,
                        onClick = { tab = t },
                        icon = { Icon(icon, contentDescription = null) },
                        label = { Text(stringResource(label)) }
                    )
                }
            }
        }
    ) { padding ->
        Box(modifier = Modifier.fillMaxSize().padding(padding)) {
            when (tab) {
                Tab.Login -> LoginScreen(onLoggedIn = { tab = Tab.Hosted })
                Tab.Hosted -> HostedScreen(locale = locale, onLocaleChange = onLocaleChange)
                Tab.Controller -> ControllerScreen()
                Tab.Admin -> AdminScreen()
            }
        }
    }
}
