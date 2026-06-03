package com.linkall.app.di

import com.linkall.app.data.api.ApiService
import com.linkall.app.data.api.AuthInterceptor
import com.linkall.app.data.db.AppDatabase
import com.linkall.app.data.repo.AdminRepo
import com.linkall.app.data.repo.AnnouncementRepo
import com.linkall.app.data.repo.AuthRepo
import com.linkall.app.data.repo.DeviceRepo
import com.linkall.app.data.repo.OtaRepo
import com.linkall.app.hosted.HostedServiceController
import com.linkall.app.util.Prefs
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import org.koin.android.ext.koin.androidContext
import org.koin.dsl.module
import retrofit2.Retrofit
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import java.util.concurrent.TimeUnit

val appModule = module {
    single { Prefs.get(androidContext()) }
    single { Json { ignoreUnknownKeys = true; encodeDefaults = true } }
    single {
        val log = HttpLoggingInterceptor().apply { level = HttpLoggingInterceptor.Level.BASIC }
        OkHttpClient.Builder()
            .addInterceptor(AuthInterceptor(get()))
            .addInterceptor(log)
            .connectTimeout(15, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .writeTimeout(30, TimeUnit.SECONDS)
            .build()
    }
    single<ApiService> {
        val prefs = get<Prefs>()
        val base = prefs.serverBase()
        Retrofit.Builder()
            .baseUrl(if (base.endsWith("/")) base else "$base/")
            .client(get())
            .addConverterFactory(get<Json>().asConverterFactory("application/json".toMediaType()))
            .build()
            .create(ApiService::class.java)
    }
    single { AppDatabase.build(androidContext()) }
    single { AuthRepo(get(), get()) }
    single { DeviceRepo(get(), get(), get()) }
    single { AnnouncementRepo(get(), get()) }
    single { OtaRepo(get(), get()) }
    single { AdminRepo(get()) }
    single { HostedServiceController(androidContext()) }
}
