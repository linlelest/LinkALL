package com.linkall.app.data.db

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.TypeConverter
import androidx.room.TypeConverters
import androidx.room.Entity
import androidx.room.PrimaryKey
import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query

@Entity(tableName = "device")
data class DeviceEntity(
    @PrimaryKey val id: Long,
    val deviceCode: String,
    val name: String?,
    val platform: String?,
    val osVersion: String?,
    val appVersion: String?,
    val allowAnonymous: Boolean,
    val requireDeviceCode: Boolean,
    val acceptConnections: Boolean,
    val lastIp: String?,
    val lastSeen: Long,
    val online: Boolean
)

@Entity(tableName = "announcement")
data class AnnouncementEntity(
    @PrimaryKey val id: Long,
    val title: String,
    val contentMd: String,
    val pinned: Boolean,
    val forceRead: Boolean,
    val createdAt: Long,
    val updatedAt: Long,
    val revoked: Boolean,
    val read: Boolean = false
)

@Dao
interface DeviceDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(d: DeviceEntity)

    @Query("SELECT * FROM device ORDER BY lastSeen DESC")
    suspend fun all(): List<DeviceEntity>

    @Query("DELETE FROM device")
    suspend fun clear()

    @Query("UPDATE device SET allowAnonymous=:a, requireDeviceCode=:r, acceptConnections=:c WHERE id=:id")
    suspend fun updateFlags(id: Long, a: Boolean, r: Boolean, c: Boolean)
}

@Dao
interface AnnouncementDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertAll(items: List<AnnouncementEntity>)

    @Query("SELECT * FROM announcement WHERE revoked=0 ORDER BY pinned DESC, id DESC")
    suspend fun list(): List<AnnouncementEntity>

    @Query("UPDATE announcement SET read=1 WHERE id=:id")
    suspend fun markRead(id: Long)

    @Query("SELECT * FROM announcement WHERE revoked=0 AND read=0 ORDER BY pinned DESC, id DESC")
    suspend fun unread(): List<AnnouncementEntity>
}

@Database(
    entities = [DeviceEntity::class, AnnouncementEntity::class],
    version = 1,
    exportSchema = false
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun devices(): DeviceDao
    abstract fun announcements(): AnnouncementDao

    companion object {
        fun build(ctx: Context): AppDatabase = Room.databaseBuilder(ctx, AppDatabase::class.java, "linkall.db")
            .fallbackToDestructiveMigration()
            .build()
    }
}
