-- =============================================
-- 通知系统相关SQL查询 (Notification Queries)
-- 对应API: 通知系统接口
-- 注意: 对应 migration/000002_first_newfunction.up.sql 中的表结构
-- =============================================

-- =============================================
-- 表结构说明 (已在migration中创建)
-- =============================================
-- notifications: 通知表
--   - notification_id (varchar 11, PK, 格式: N0000000001)
--   - user_id (varchar 10, FK -> users)
--   - type (notification_type ENUM: friend/chatroom/system)
--   - title (varchar 255)
--   - content (TEXT)
--   - data (JSONB)
--   - is_read (boolean)
--   - created_at (TIMESTAMPTZ)

-- =============================================
-- 1. 通知创建 (Notification Creation)
-- =============================================

-- name: CreateNotification :one
-- 创建通知
INSERT INTO notifications (
    user_id,
    type,
    title,
    content,
    data
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING 
    notification_id,
    user_id,
    type,
    title,
    content,
    data,
    is_read,
    created_at;

-- name: CreateFriendNotification :one
-- 创建好友相关通知
INSERT INTO notifications (
    user_id,
    type,
    title,
    content,
    data
) VALUES (
    $1, 'friend', $2, $3, $4
) RETURNING 
    notification_id,
    user_id,
    type,
    title,
    content,
    data,
    is_read,
    created_at;

-- name: CreateChatroomNotification :one
-- 创建聊天室相关通知
INSERT INTO notifications (
    user_id,
    type,
    title,
    content,
    data
) VALUES (
    $1, 'chatroom', $2, $3, $4
) RETURNING 
    notification_id,
    user_id,
    type,
    title,
    content,
    data,
    is_read,
    created_at;

-- name: CreateSystemNotification :one
-- 创建系统通知
INSERT INTO notifications (
    user_id,
    type,
    title,
    content,
    data
) VALUES (
    $1, 'system', $2, $3, $4
) RETURNING 
    notification_id,
    user_id,
    type,
    title,
    content,
    data,
    is_read,
    created_at;

-- 注意: 批量创建通知需要在应用层使用循环调用 CreateNotification
-- 或者使用事务处理多次插入

-- =============================================
-- 2. 通知查询 (Notification Queries)
-- =============================================

-- name: GetNotificationByID :one
-- 获取单条通知
SELECT 
    notification_id,
    user_id,
    type,
    title,
    content,
    data,
    is_read,
    created_at
FROM notifications 
WHERE notification_id = $1;

-- name: GetUserNotifications :many
-- 获取用户通知列表 GET /users/me/notifications
SELECT 
    notification_id,
    user_id,
    type,
    title,
    content,
    data,
    is_read,
    created_at
FROM notifications 
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUserNotificationsByType :many
-- 按类型获取用户通知 GET /users/me/notifications?type=friend
SELECT 
    notification_id,
    user_id,
    type,
    title,
    content,
    data,
    is_read,
    created_at
FROM notifications 
WHERE user_id = $1 AND type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetUnreadNotifications :many
-- 获取未读通知 GET /users/me/notifications?status=unread
SELECT 
    notification_id,
    user_id,
    type,
    title,
    content,
    data,
    is_read,
    created_at
FROM notifications 
WHERE user_id = $1 AND is_read = false
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUnreadNotificationsByType :many
-- 按类型获取未读通知
SELECT 
    notification_id,
    user_id,
    type,
    title,
    content,
    data,
    is_read,
    created_at
FROM notifications 
WHERE user_id = $1 AND type = $2 AND is_read = false
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- =============================================
-- 3. 通知状态管理 (Notification Status Management)
-- =============================================

-- name: MarkNotificationAsRead :exec
-- 标记通知已读 POST /notifications/:notificationId/read
UPDATE notifications 
SET is_read = true
WHERE notification_id = $1 AND user_id = $2;

-- name: MarkAllNotificationsAsRead :exec
-- 标记所有通知已读 POST /users/me/notifications/read-all
UPDATE notifications 
SET is_read = true
WHERE user_id = $1 AND is_read = false;

-- name: MarkNotificationsByTypeAsRead :exec
-- 按类型标记通知已读
UPDATE notifications 
SET is_read = true
WHERE user_id = $1 AND type = $2 AND is_read = false;

-- name: DeleteNotification :exec
-- 删除通知
DELETE FROM notifications 
WHERE notification_id = $1 AND user_id = $2;

-- name: DeleteReadNotifications :exec
-- 删除已读通知
DELETE FROM notifications 
WHERE user_id = $1 AND is_read = true;

-- name: DeleteOldNotifications :exec
-- 删除超过指定天数的通知
DELETE FROM notifications 
WHERE created_at < NOW() - ($1 || ' days')::INTERVAL;

-- =============================================
-- 4. 通知统计 (Notification Statistics)
-- =============================================

-- name: CountUserNotifications :one
-- 统计用户通知总数
SELECT COUNT(*) 
FROM notifications 
WHERE user_id = $1;

-- name: CountUnreadNotifications :one
-- 统计未读通知数量
SELECT COUNT(*) 
FROM notifications 
WHERE user_id = $1 AND is_read = false;

-- name: CountUnreadNotificationsByType :one
-- 按类型统计未读通知数量
SELECT COUNT(*) 
FROM notifications 
WHERE user_id = $1 AND type = $2 AND is_read = false;

-- name: GetNotificationStats :one
-- 获取通知统计（总数、未读数、各类型未读数）
SELECT 
    COUNT(*) AS total_count,
    COUNT(*) FILTER (WHERE is_read = false) AS unread_count,
    COUNT(*) FILTER (WHERE type = 'friend' AND is_read = false) AS friend_unread,
    COUNT(*) FILTER (WHERE type = 'chatroom' AND is_read = false) AS chatroom_unread,
    COUNT(*) FILTER (WHERE type = 'system' AND is_read = false) AS system_unread
FROM notifications 
WHERE user_id = $1;
