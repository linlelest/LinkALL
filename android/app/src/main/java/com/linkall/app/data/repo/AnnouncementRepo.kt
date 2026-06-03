package com.linkall.app.data.repo

import com.linkall.app.data.api.ApiService
import com.linkall.app.data.db.AnnouncementEntity
import com.linkall.app.data.db.AppDatabase

class AnnouncementRepo(
    private val api: ApiService,
    private val db: AppDatabase
) {
    private val dao get() = db.announcements()

    suspend fun refresh() {
        val list = api.listAnnouncements(includeRevoked = false)
        dao.upsertAll(list.map { it.toEntity() })
    }

    suspend fun list() = dao.list()
    suspend fun unread() = dao.unread()
    suspend fun markRead(id: Long) {
        dao.markRead(id)
        runCatching { api.markAnnouncementRead(id) }
    }

    private fun com.linkall.app.data.api.Announcement.toEntity() = AnnouncementEntity(
        id = id,
        title = title,
        contentMd = contentMd,
        pinned = pinned,
        forceRead = forceRead,
        createdAt = createdAt,
        updatedAt = updatedAt,
        revoked = revoked
    )
}
