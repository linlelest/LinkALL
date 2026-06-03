package com.linkall.app.ui.login

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import com.linkall.app.R
import com.linkall.app.data.repo.AuthRepo
import com.linkall.app.ui.components.Card
import com.linkall.app.ui.components.GhostButton
import com.linkall.app.ui.components.PrimaryButton
import com.linkall.app.util.Prefs
import kotlinx.coroutines.launch
import org.koin.compose.koinInject

@Composable
fun LoginScreen(onLoggedIn: () -> Unit) {
    val repo: AuthRepo = koinInject()
    val prefs: Prefs = koinInject()
    val scope = rememberCoroutineScope()

    var action by remember { mutableStateOf("login") }
    var username by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var invite by remember { mutableStateOf("") }
    var server by remember { mutableStateOf(prefs.serverOverride ?: "http://127.0.0.1:8080") }
    var err by remember { mutableStateOf<String?>(null) }
    var loading by remember { mutableStateOf(false) }

    Column(
        modifier = Modifier.fillMaxSize().padding(20.dp),
        verticalArrangement = Arrangement.Center
    ) {
        Text("LinkALL", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.primary)
        Text(stringResource(R.string.app_tagline), color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(20.dp))

        Card {
            Row {
                TextButton(onClick = { action = "login" }) { Text(stringResource(R.string.login_tab_login), color = if (action == "login") MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurface) }
                TextButton(onClick = { action = "register" }) { Text(stringResource(R.string.login_tab_register), color = if (action == "register") MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurface) }
            }
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(value = username, onValueChange = { username = it }, label = { Text(stringResource(R.string.login_username)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(value = password, onValueChange = { password = it }, label = { Text(stringResource(R.string.login_password)) }, singleLine = true, visualTransformation = PasswordVisualTransformation(), modifier = Modifier.fillMaxWidth())
            if (action == "register") {
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(value = invite, onValueChange = { invite = it }, label = { Text(stringResource(R.string.login_invite)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            }
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(value = server, onValueChange = { server = it }, label = { Text(stringResource(R.string.login_server)) }, singleLine = true, modifier = Modifier.fillMaxWidth())

            if (err != null) {
                Spacer(Modifier.height(8.dp))
                Text(err!!, color = MaterialTheme.colorScheme.error)
            }

            Spacer(Modifier.height(12.dp))
            if (action == "login") {
                PrimaryButton(text = stringResource(R.string.login_submit), enabled = !loading, modifier = Modifier.fillMaxWidth()) {
                    err = null
                    loading = true
                    prefs.setServerBase(server)
                    scope.launch {
                        runCatching { repo.login(username.trim(), password) }
                            .onSuccess { onLoggedIn() }
                            .onFailure { err = it.message ?: "login failed" }
                        loading = false
                    }
                }
            } else {
                PrimaryButton(text = stringResource(R.string.login_register), enabled = !loading, modifier = Modifier.fillMaxWidth()) {
                    err = null
                    loading = true
                    prefs.setServerBase(server)
                    scope.launch {
                        runCatching { repo.register(username.trim(), password, invite.trim()) }
                            .onSuccess { onLoggedIn() }
                            .onFailure { err = it.message ?: "register failed" }
                        loading = false
                    }
                }
            }
        }
    }
}
