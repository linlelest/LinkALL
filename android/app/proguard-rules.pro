# LinkALL Android ProGuard / R8 rules
# 目的：release 构建启用 R8 full mode + minify 后，保留所有反射 / native / 序列化相关符号

# === 通用 ===
-keepattributes *Annotation*,Signature,InnerClasses,EnclosingMethod,SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile

# === Kotlinx Serialization（必须保留 @Serializable 类的 fields 和 constructor）===
-keepattributes RuntimeVisibleAnnotations,AnnotationDefault
-keep,includedescriptorclasses class **$$serializer { *; }
-keepclassmembers class ** {
    *** Companion;
}
-keepclasseswithmembers class ** {
    kotlinx.serialization.KSerializer serializer(...);
}
# kotlinx.serialization 反射
-keep class kotlinx.serialization.** { *; }
-keepclassmembers,allowobfuscation class * {
    @kotlinx.serialization.Serializable <fields>;
}
# 避免 R8 优化掉泛型类型信息（Retrofit / OkHttp 需要）
-keepattributes Signature

# === Retrofit + OkHttp ===
-keep,allowobfuscation,allowshrinking interface retrofit2.Call
-keep,allowobfuscation,allowshrinking class retrofit2.Response
-keepclasseswithmembers,includedescriptorclasses interface * {
    @retrofit2.http.* <methods>;
}
-dontwarn retrofit2.**
-dontwarn okhttp3.**
-dontwarn okio.**
-dontwarn org.conscrypt.**
-dontwarn org.openjsse.**
-dontwarn org.bouncycastle.**
-keep class okhttp3.** { *; }
-keep class okio.** { *; }
-keep class org.codehaus.mojo.animal_sniffer.** { *; }
-keep class javax.annotation.** { *; }

# === Room（如果启用）===
-keep class * extends androidx.room.RoomDatabase
-keep @androidx.room.Entity class *
-dontwarn androidx.room.paging.**

# === Koin DI（反射注入）===
-keep class io.insert-koin.** { *; }
-keep class org.koin.** { *; }
-keepclassmembers class * {
    @org.koin.core.annotation.* *;
}
# 我们的 Koin module（com.linkall.app.di.AppModule）
-keep class com.linkall.app.di.** { *; }

# === WebRTC（libwebrtc 原生库 + Java 桥接）===
# 整个 org.webrtc 包都用 JNI，必须保留所有类和 native method
-keep class org.webrtc.** { *; }
-keepclasseswithmembernames class * {
    native <methods>;
}
# JNI 调用 Java 端 callback
-keepclassmembers class org.webrtc.** {
    public *;
    protected *;
}
# 避免 R8 重命名 native lib 引用
-keepclasseswithmembernames,includedescriptorclasses class * {
    native <methods>;
}

# === androidx.security.crypto（EncryptedSharedPreferences / Tink）===
-keep class androidx.security.crypto.** { *; }
-keep class com.google.crypto.tink.** { *; }
-dontwarn com.google.errorprone.annotations.**
-keep class com.google.errorprone.annotations.** { *; }

# === 我们的项目（被反射调用的关键类） ===
# 1. Prefs（Kotlin 单例 + @JvmStatic）
-keep class com.linkall.app.util.Prefs { *; }
-keep class com.linkall.app.util.Prefs$Companion { *; }
# 2. InputInjector / KeyMap（被 IME 反射）
-keep class com.linkall.app.controller.InputInjector { *; }
-keep class com.linkall.app.controller.InputInjector$* { *; }
-keep class com.linkall.app.controller.KeyMap { *; }
-keep class com.linkall.app.controller.KeyMap$* { *; }
# 3. HostedRtcSession / WebRtcHost（Kotlin object）
-keep class com.linkall.app.hosted.HostedRtcSession { *; }
-keep class com.linkall.app.hosted.HostedRtcSession$* { *; }
-keep class com.linkall.app.hosted.WebRtcHost { *; }
-keep class com.linkall.app.hosted.WebRtcHost$* { *; }
-keep class com.linkall.app.hosted.LinkALLIme { *; }
-keep class com.linkall.app.hosted.LinkALLIme$* { *; }
-keep class com.linkall.app.hosted.ImeRegistry { *; }
-keep class com.linkall.app.hosted.ImeRegistry$* { *; }
-keep class com.linkall.app.hosted.ScreenCaptureService { *; }
# 4. WebRtcController（控制端 DataChannel 接收 + 文件事件）
-keep class com.linkall.app.controller.WebRtcController { *; }
-keep class com.linkall.app.controller.WebRtcController$* { *; }
# 5. JSON / Data model（API 响应解析）
-keep class com.linkall.app.data.api.** { *; }
-keep class com.linkall.app.data.repo.** { *; }
-keep class com.linkall.app.data.db.** { *; }

# === Enum（Kotlin enum 用 values() / valueOf() 反射） ===
-keepclassmembers enum * {
    public static **[] values();
    public static ** valueOf(java.lang.String);
}

# === Parcelable（Intent extras 序列化） ===
-keepclassmembers class * implements android.os.Parcelable {
    public static final android.os.Parcelable$Creator CREATOR;
}

# === Serializable（Kotlin/Java 标准） ===
-keepclassmembers class * implements java.io.Serializable {
    static final long serialVersionUID;
    private static final java.io.ObjectStreamField[] serialPersistentFields;
    !static !transient <fields>;
    private void writeObject(java.io.ObjectOutputStream);
    private void readObject(java.io.ObjectInputStream);
    java.lang.Object writeReplace();
    java.lang.Object readResolve();
}

# === Activity / Service 入口（AndroidManifest 注册的） ===
-keep public class com.linkall.app.LinkALLApp
-keep public class com.linkall.app.ui.MainActivity
-keep public class com.linkall.app.hosted.ScreenCaptureService
-keep public class com.linkall.app.hosted.LinkALLIme
-keep public class com.linkall.app.hosted.LinkALLAccessibilityService
-keep public class com.linkall.app.hosted.BootReceiver
-keep public class com.linkall.app.push.LinkALLMessagingService
-keep public class * extends android.app.Service
-keep public class * extends android.content.BroadcastReceiver
-keep public class * extends android.content.ContentProvider

# === FCM (optional - 没有 google-services.json 时不会运行) ===
-keep class com.google.firebase.** { *; }
-keep class com.google.android.gms.** { *; }
-keep class com.google.firebase.messaging.FirebaseMessagingService
-dontwarn com.google.firebase.**

# === R8 full mode 友好 ===
-allowaccessmodification
-repackageclasses 'l'
-overloadaggressively

# === 警告压制（非 fatal）===
-dontwarn java.lang.invoke.**
-dontwarn org.jetbrains.annotations.**
