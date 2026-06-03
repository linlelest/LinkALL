package com.linkall.app.data.api

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.Multipart
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.Part
import retrofit2.http.Path
import retrofit2.http.Query

// ============== Auth ==============
@Serializable
data class LoginReq(
    val action: String,                 // "login" | "register"
    val username: String,
    val password: String,
    @SerialName("invite_code") val inviteCode: String? = null
)

@Serializable
data class LoginResp(
    val token: String,
    val user: User
)

@Serializable
data class User(
    val id: Long,
    val username: String,
    @SerialName("is_admin") val isAdmin: Boolean = false,
    @SerialName("is_super_admin") val isSuperAdmin: Boolean = false,
    val locale: String? = null,
    val avatar: String? = null
)

// ============== Devices ==============
@Serializable
data class RegisterDeviceReq(
    @SerialName("device_code") val deviceCode: String,
    @SerialName("device_password") val devicePassword: String,
    val name: String? = null,
    val platform: String? = null,
    @SerialName("os_version") val osVersion: String? = null,
    @SerialName("app_version") val appVersion: String? = null,
    @SerialName("allow_anonymous") val allowAnonymous: Boolean? = null,
    @SerialName("require_device_code") val requireDeviceCode: Boolean? = null,
    @SerialName("accept_connections") val acceptConnections: Boolean? = null
)

@Serializable
data class LoginDeviceReq(
    @SerialName("device_code") val deviceCode: String,
    @SerialName("device_password") val devicePassword: String
)

@Serializable
data class DeviceRecord(
    val id: Long,
    @SerialName("device_code") val deviceCode: String,
    val name: String? = null,
    val platform: String? = null,
    @SerialName("os_version") val osVersion: String? = null,
    @SerialName("app_version") val appVersion: String? = null,
    @SerialName("allow_anonymous") val allowAnonymous: Boolean = true,
    @SerialName("require_device_code") val requireDeviceCode: Boolean = true,
    @SerialName("accept_connections") val acceptConnections: Boolean = true,
    @SerialName("last_ip") val lastIp: String? = null,
    @SerialName("last_seen") val lastSeen: Long = 0,
    @SerialName("created_at") val createdAt: Long = 0,
    val online: Boolean = false,
    val tag: String? = null,
    val notes: String? = null,
    val token: String? = null
)

@Serializable
data class UpdateDeviceReq(
    val name: String? = null,
    @SerialName("allow_anonymous") val allowAnonymous: Boolean? = null,
    @SerialName("require_device_code") val requireDeviceCode: Boolean? = null,
    @SerialName("accept_connections") val acceptConnections: Boolean? = null,
    val tag: String? = null,
    val notes: String? = null
)

@Serializable
data class ResetDeviceReq(
    @SerialName("new_code") val newCode: String = "",
    @SerialName("new_password") val newPassword: String
)

// ============== Announcements ==============
@Serializable
data class Announcement(
    val id: Long,
    @SerialName("author_id") val authorId: Long = 0,
    @SerialName("author_name") val authorName: String? = null,
    val title: String,
    @SerialName("content_md") val contentMd: String,
    val platform: String? = null,
    @SerialName("min_version") val minVersion: String? = null,
    val pinned: Boolean = false,
    @SerialName("force_read") val forceRead: Boolean = false,
    val signature: String? = null,
    @SerialName("created_at") val createdAt: Long = 0,
    @SerialName("updated_at") val updatedAt: Long = 0,
    val revoked: Boolean = false
)

// ============== OTA ==============
@Serializable
data class OtaInfo(
    val has_update: Boolean = false,
    @SerialName("force_update") val forceUpdate: Boolean = false,
    val platform: String? = null,
    val version: String? = null,
    val channel: String? = null,
    @SerialName("release_notes") val releaseNotes: String? = null,
    @SerialName("min_supported_version") val minSupportedVersion: String? = null,
    @SerialName("file_size") val fileSize: Long = 0,
    val sha256: String? = null,
    val signature: String? = null,
    @SerialName("download_url") val downloadUrl: String? = null,
    @SerialName("created_at") val createdAt: Long = 0
)

@Serializable
data class OtaPackage(
    val id: Long,
    val platform: String,
    val version: String,
    val channel: String = "stable",
    @SerialName("file_name") val fileName: String = "",
    @SerialName("file_size") val fileSize: Long = 0,
    val sha256: String = "",
    val signature: String? = null,
    @SerialName("release_notes") val releaseNotes: String? = null,
    @SerialName("force_update") val forceUpdate: Boolean = false,
    @SerialName("min_supported_version") val minSupportedVersion: String? = null,
    val downloads: Long = 0,
    @SerialName("created_at") val createdAt: Long = 0,
    val revoked: Boolean = false
)

// ============== Admin ==============
@Serializable
data class AdminStats(
    val users: Int = 0,
    val devices: Int = 0,
    val online: Int = 0,
    val sessions: Int = 0,
    @SerialName("sessions_total") val sessionsTotal: Long = 0,
    @SerialName("bytes_tx") val bytesTx: Long = 0,
    @SerialName("bytes_rx") val bytesRx: Long = 0,
    @SerialName("server_time") val serverTime: Long = 0,
    @SerialName("go_version") val goVersion: String? = null,
    @SerialName("go_routines") val goRoutines: Int = 0,
    @SerialName("go_mem_alloc") val goMemAlloc: Long = 0
)

@Serializable
data class AdminUser(
    val id: Long,
    val username: String,
    @SerialName("is_admin") val isAdmin: Boolean = false,
    @SerialName("is_super_admin") val isSuperAdmin: Boolean = false,
    val banned: Boolean = false,
    @SerialName("last_login_ip") val lastLoginIp: String? = null,
    @SerialName("last_login_at") val lastLoginAt: Long = 0
)

@Serializable
data class AdminUpdateUserReq(
    val banned: Boolean? = null,
    @SerialName("is_admin") val isAdmin: Boolean? = null,
    @SerialName("is_super_admin") val isSuperAdmin: Boolean? = null,
    @SerialName("new_password") val newPassword: String? = null
)

@Serializable
data class InviteCode(
    val id: Long,
    val code: String,
    @SerialName("max_uses") val maxUses: Int,
    @SerialName("used_count") val usedCount: Int,
    @SerialName("ttl_hours") val ttlHours: Int,
    @SerialName("expires_at") val expiresAt: Long,
    val revoked: Boolean = false,
    val note: String? = null
)

@Serializable
data class CreateInviteReq(
    @SerialName("max_uses") val maxUses: Int = 1,
    @SerialName("ttl_hours") val ttlHours: Int = 72,
    val note: String? = null
)

// ============== API ==============
interface ApiService {
    @GET("api/config")
    suspend fun getConfig(): Map<String, @kotlinx.serialization.Contextual Any>

    // Auth
    @POST("api/auth/login")
    suspend fun login(@Body req: LoginReq): LoginResp
    @GET("api/auth/me")
    suspend fun me(): User
    @POST("api/auth/password")
    suspend fun changePassword(@Body req: Map<String, String>): Map<String, String>
    @POST("api/auth/locale")
    suspend fun setLocale(@Body req: Map<String, String>): Map<String, String>

    // Devices
    @POST("api/devices/register")
    suspend fun registerDevice(@Body req: RegisterDeviceReq): Map<String, @kotlinx.serialization.Contextual Any>
    @POST("api/devices/login")
    suspend fun loginDevice(@Body req: LoginDeviceReq): Map<String, @kotlinx.serialization.Contextual Any>
    @GET("api/devices")
    suspend fun listDevices(): List<DeviceRecord>
    @GET("api/devices/{id}")
    suspend fun getDevice(@Path("id") id: Long): DeviceRecord
    @PATCH("api/devices/{id}")
    suspend fun updateDevice(@Path("id") id: Long, @Body req: UpdateDeviceReq): DeviceRecord
    @POST("api/devices/{id}/reset-code")
    suspend fun resetDeviceCode(@Path("id") id: Long, @Body req: ResetDeviceReq): DeviceRecord
    @DELETE("api/devices/{id}")
    suspend fun deleteDevice(@Path("id") id: Long): Map<String, String>

    // Announcements
    @GET("api/announcements")
    suspend fun listAnnouncements(@Query("include_revoked") includeRevoked: Boolean = false): List<Announcement>
    @GET("api/announcements/unread")
    suspend fun unreadAnnouncements(): List<Announcement>
    @GET("api/announcements/{id}")
    suspend fun getAnnouncement(@Path("id") id: Long): Announcement
    @POST("api/announcements/{id}/read")
    suspend fun markAnnouncementRead(@Path("id") id: Long): Map<String, String>

    // OTA
    @GET("api/ota/check")
    suspend fun checkOta(@Query("platform") platform: String, @Query("version") version: String): OtaInfo
    @GET("api/ota/list")
    suspend fun listOta(@Query("include_revoked") includeRevoked: Boolean = false): List<OtaPackage>

    // Admin
    @GET("api/admin/stats")
    suspend fun adminStats(): AdminStats
    @GET("api/admin/users")
    suspend fun adminUsers(): List<AdminUser>
    @PATCH("api/admin/users/{id}")
    suspend fun adminUpdateUser(@Path("id") id: Long, @Body req: AdminUpdateUserReq): Map<String, String>
    @DELETE("api/admin/users/{id}")
    suspend fun adminDeleteUser(@Path("id") id: Long): Map<String, String>

    @GET("api/invites")
    suspend fun listInvites(): List<InviteCode>
    @POST("api/invites")
    suspend fun createInvite(@Body req: CreateInviteReq): InviteCode
    @DELETE("api/invites/{id}")
    suspend fun revokeInvite(@Path("id") id: Long): Map<String, String>
}
