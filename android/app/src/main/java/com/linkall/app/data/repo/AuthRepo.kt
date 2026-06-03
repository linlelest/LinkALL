package com.linkall.app.data.repo

import com.linkall.app.data.api.ApiService
import com.linkall.app.data.api.LoginReq
import com.linkall.app.data.api.User
import com.linkall.app.util.Prefs

class AuthRepo(
    private val api: ApiService,
    private val prefs: Prefs
) {
    suspend fun login(username: String, password: String): User {
        val r = api.login(LoginReq("login", username, password))
        prefs.token = r.token
        prefs.username = r.user.username
        return r.user
    }

    suspend fun register(username: String, password: String, invite: String): User {
        val r = api.login(LoginReq("register", username, password, invite))
        prefs.token = r.token
        prefs.username = r.user.username
        return r.user
    }

    fun logout() {
        prefs.clearAuth()
    }

    suspend fun me(): User = api.me()

    suspend fun setLocale(locale: String) {
        prefs.locale = locale
        runCatching { api.setLocale(mapOf("locale" to locale)) }
    }
}
