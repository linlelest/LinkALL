package com.linkall.app.data.repo

import com.linkall.app.data.api.AdminStats
import com.linkall.app.data.api.Announcement
import com.linkall.app.data.api.ApiService
import com.linkall.app.data.api.OtaInfo

class AdminRepo(private val api: ApiService) {
    suspend fun stats(): AdminStats = api.adminStats()
    suspend fun ota(): OtaInfo = api.checkOta("android-arm64", "1.0.0")
    suspend fun announcements(): List<Announcement> = api.listAnnouncements(false)
}
