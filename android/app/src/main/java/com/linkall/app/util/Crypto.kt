package com.linkall.app.util

import java.security.SecureRandom

object Crypto {
    private val rng = SecureRandom()

    fun randomCode12(): String {
        val src = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"
        return buildString { repeat(12) { append(src[rng.nextInt(src.length)]) } }
    }

    fun randomNumeric(n: Int): String {
        val src = "0123456789"
        return buildString { repeat(n) { append(src[rng.nextInt(src.length)]) } }
    }
}
