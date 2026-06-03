package com.linkall.app.data.repo

import com.linkall.app.data.api.ApiService
import com.linkall.app.data.api.LoginDeviceReq
import com.linkall.app.data.api.RegisterDeviceReq
import com.linkall.app.data.api.ResetDeviceReq
import com.linkall.app.data.api.UpdateDeviceReq
import com.linkall.app.data.db.AppDatabase
import com.linkall.app.data.db.DeviceEntity
import com.linkall.app.util.Prefs

class DeviceRepo(
    private val api: ApiService,
    private val prefs: Prefs
) {
    private val dao get() = db.devices()

    suspend fun register(code: String, password: String, name: String?): DeviceEntity {
        val r = api.registerDevice(RegisterDeviceReq(
            deviceCode = code,
            devicePassword = password,
            name = name,
            platform = "android-arm64",
            osVersion = "Android ${android.os.Build.VERSION.RELEASE} (API ${android.os.Build.VERSION.SDK_INT})",
            appVersion = "1.0.0"
        ))
        val dev = (r["device"] as Map<*, *>)
        val token = r["token"] as String
        val ent = dev.toEntity(token)
        dao.upsert(ent)
        prefs.deviceCode = ent.deviceCode
        prefs.deviceToken = token
        prefs.devicePassword = password
        return ent
    }

    suspend fun login(code: String, password: String): DeviceEntity {
        val r = api.loginDevice(LoginDeviceReq(code, password))
        val dev = r["device"] as Map<*, *>
        val token = r["token"] as String
        val ent = dev.toEntity(token)
        dao.upsert(ent)
        prefs.deviceCode = ent.deviceCode
        prefs.deviceToken = token
        prefs.devicePassword = password
        return ent
    }

    suspend fun updateFlags(id: Long, allow: Boolean, req: Boolean, accept: Boolean) {
        api.updateDevice(id, UpdateDeviceReq(
            allowAnonymous = allow,
            requireDeviceCode = req,
            acceptConnections = accept
        ))
        dao.updateFlags(id, allow, req, accept)
    }

    suspend fun resetCode(id: Long, newCode: String, newPassword: String): DeviceEntity {
        val d = api.resetDeviceCode(id, ResetDeviceReq(newCode, newPassword))
        prefs.deviceCode = d.deviceCode
        prefs.devicePassword = newPassword
        val ent = DeviceEntity(
            id = d.id, deviceCode = d.deviceCode, name = d.name, platform = d.platform,
            osVersion = d.osVersion, appVersion = d.appVersion,
            allowAnonymous = d.allowAnonymous, requireDeviceCode = d.requireDeviceCode,
            acceptConnections = d.acceptConnections, lastIp = d.lastIp, lastSeen = d.lastSeen, online = d.online
        )
        dao.upsert(ent)
        return ent
    }

    suspend fun list() = api.listDevices()
    suspend fun logout() {
        prefs.clearDevice()
        dao.clear()
    }

    private fun Map<*, *>.toEntity(token: String): DeviceEntity {
        return DeviceEntity(
            id = (this["id"] as Number).toLong(),
            deviceCode = this["device_code"] as String,
            name = this["name"] as? String,
            platform = this["platform"] as? String,
            osVersion = this["os_version"] as? String,
            appVersion = this["app_version"] as? String,
            allowAnonymous = this["allow_anonymous"] as? Boolean ?: true,
            requireDeviceCode = this["require_device_code"] as? Boolean ?: true,
            acceptConnections = this["accept_connections"] as? Boolean ?: true,
            lastIp = this["last_ip"] as? String,
            lastSeen = (this["last_seen"] as? Number)?.toLong() ?: 0L,
            online = this["online"] as? Boolean ?: false
        )
    }
}
