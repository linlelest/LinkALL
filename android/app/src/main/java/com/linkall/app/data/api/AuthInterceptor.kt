package com.linkall.app.data.api

import com.linkall.app.util.Prefs
import okhttp3.Interceptor
import okhttp3.Response

class AuthInterceptor(private val prefs: Prefs) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val req = chain.request()
        val token = prefs.token ?: prefs.deviceToken
        val newReq = if (!token.isNullOrEmpty()) {
            req.newBuilder().addHeader("Authorization", "Bearer $token").build()
        } else req
        return chain.proceed(newReq)
    }
}
