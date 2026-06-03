package com.linkall.app.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.linkall.app.ui.theme.Border
import com.linkall.app.ui.theme.Muted
import com.linkall.app.ui.theme.Panel
import com.linkall.app.ui.theme.Primary

@Composable
fun PrimaryButton(text: String, modifier: Modifier = Modifier, enabled: Boolean = true, onClick: () -> Unit) {
    Button(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier,
        colors = ButtonDefaults.buttonColors(containerColor = Primary)
    ) { Text(text) }
}

@Composable
fun GhostButton(text: String, modifier: Modifier = Modifier, enabled: Boolean = true, onClick: () -> Unit) {
    OutlinedButton(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier
    ) { Text(text) }
}

@Composable
fun LabeledSwitch(label: String, value: Boolean, onChange: (Boolean) -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Text(label, modifier = Modifier.weight(1f), color = MaterialTheme.colorScheme.onSurface)
        Switch(checked = value, onCheckedChange = onChange, colors = SwitchDefaults.colors())
    }
}

@Composable
fun Card(content: @Composable () -> Unit) {
    Box(
        Modifier
            .fillMaxWidth()
            .background(Panel, RoundedCornerShape(12.dp))
            .border(1.dp, Border, RoundedCornerShape(12.dp))
            .padding(12.dp)
    ) { content() }
}

@Composable
fun MonoText(text: String, color: Color = MaterialTheme.colorScheme.onSurface) {
    Text(text, color = color, fontFamily = FontFamily.Monospace, fontSize = 13.sp)
}

@Composable
fun Divider() {
    Box(Modifier.fillMaxWidth().height(1.dp).background(Border))
}

@Composable
fun MutedText(text: String) {
    Text(text, color = Muted, fontSize = 12.sp)
}

@Composable
fun StatusDot(online: Boolean) {
    val c = if (online) Color(0xFF10B981) else Muted
    Box(Modifier.padding(end = 6.dp).background(c, RoundedCornerShape(50)).padding(2.dp)) {
        Box(Modifier.background(c, RoundedCornerShape(50)))
    }
}
