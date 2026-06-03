package com.linkall.app.data.repo

import com.linkall.app.data.api.ApiService
import com.linkall.app.data.api.OtaInfo
import com.linkall.app.util.Prefs

class OtaRepo(
    private val api: ApiService,
    private val prefs: Prefs
) {
    suspend fun check(platform: String = "android-arm64", version: String = "1.0.0"): OtaInfo =
        api.checkOta(platform, version)

    suspend fun list() = api.listOta(includeRevoked = true)
}
