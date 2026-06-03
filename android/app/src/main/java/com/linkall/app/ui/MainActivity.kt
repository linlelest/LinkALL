package com.linkall.app.ui

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import com.linkall.app.ui.nav.RootScaffold
import com.linkall.app.ui.theme.LinkALLTheme
import com.linkall.app.util.Prefs
import org.koin.android.ext.android.inject

class MainActivity : ComponentActivity() {
    private val prefs: Prefs by inject()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            LinkALLTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    AppRoot(prefs)
                }
            }
        }
    }
}

@Composable
fun AppRoot(prefs: Prefs) {
    var locale by remember { mutableStateOf(prefs.locale) }
    LaunchedEffect(locale) { prefs.locale = locale }
    RootScaffold(locale = locale, onLocaleChange = { locale = it })
}
