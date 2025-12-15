-- =============================================
-- 用户设置相关SQL查询 (User Settings Queries)
-- 对应API: 用户设置接口
-- 注意: 对应 migration/000002_first_newfunction.up.sql 中的表结构
-- =============================================

-- =============================================
-- 表结构说明 (已在migration中创建)
-- =============================================
-- user_settings: 用户设置表
--   - user_id (varchar 10, PK, FK -> users)
--   - enable_friend_request (boolean)
--   - enable_chatroom_messages (boolean) ← 注意是复数
--   - enable_system_notice (boolean)
--   - enable_sound (boolean)
--   - enable_desktop_notification (boolean)
--   - allow_search_by_phone (boolean)
--   - allow_search_by_email (boolean)
--   - show_online_status (boolean)
--   - update_at (TIMESTAMPTZ) ← 注意字段名

-- =============================================
-- 1. 用户设置操作 (User Settings Operations)
-- =============================================

-- name: CreateUserSettings :one
-- 创建用户设置（用户注册时调用）
INSERT INTO user_settings (user_id) 
VALUES ($1)
RETURNING 
    user_id,
    enable_friend_request,
    enable_chatroom_messages,
    enable_system_notice,
    enable_sound,
    enable_desktop_notification,
    allow_search_by_phone,
    allow_search_by_email,
    show_online_status,
    update_at;

-- name: GetUserSettings :one
-- 获取用户设置 GET /users/me/settings
SELECT 
    user_id,
    enable_friend_request,
    enable_chatroom_messages,
    enable_system_notice,
    enable_sound,
    enable_desktop_notification,
    allow_search_by_phone,
    allow_search_by_email,
    show_online_status,
    update_at
FROM user_settings 
WHERE user_id = $1;

-- name: UpdateUserSettings :one
-- 更新用户设置 POST /users/me/settings/update
UPDATE user_settings 
SET 
    enable_friend_request = COALESCE($2, enable_friend_request),
    enable_chatroom_messages = COALESCE($3, enable_chatroom_messages),
    enable_system_notice = COALESCE($4, enable_system_notice),
    enable_sound = COALESCE($5, enable_sound),
    enable_desktop_notification = COALESCE($6, enable_desktop_notification),
    allow_search_by_phone = COALESCE($7, allow_search_by_phone),
    allow_search_by_email = COALESCE($8, allow_search_by_email),
    show_online_status = COALESCE($9, show_online_status),
    update_at = NOW()
WHERE user_id = $1
RETURNING 
    user_id,
    enable_friend_request,
    enable_chatroom_messages,
    enable_system_notice,
    enable_sound,
    enable_desktop_notification,
    allow_search_by_phone,
    allow_search_by_email,
    show_online_status,
    update_at;

-- name: UpdateNotificationSettings :exec
-- 更新通知设置
UPDATE user_settings 
SET 
    enable_friend_request = COALESCE($2, enable_friend_request),
    enable_chatroom_messages = COALESCE($3, enable_chatroom_messages),
    enable_system_notice = COALESCE($4, enable_system_notice),
    enable_sound = COALESCE($5, enable_sound),
    enable_desktop_notification = COALESCE($6, enable_desktop_notification),
    update_at = NOW()
WHERE user_id = $1;

-- name: UpdatePrivacySettings :exec
-- 更新隐私设置
UPDATE user_settings 
SET 
    allow_search_by_phone = COALESCE($2, allow_search_by_phone),
    allow_search_by_email = COALESCE($3, allow_search_by_email),
    show_online_status = COALESCE($4, show_online_status),
    update_at = NOW()
WHERE user_id = $1;

-- name: DeleteUserSettings :exec
-- 删除用户设置（用户注销时调用）
DELETE FROM user_settings 
WHERE user_id = $1;

-- =============================================
-- 2. 设置查询辅助 (Settings Query Helpers)
-- =============================================

-- name: IsUserSearchableByPhone :one
-- 检查用户是否允许通过手机号搜索
SELECT allow_search_by_phone 
FROM user_settings 
WHERE user_id = $1;

-- name: IsUserSearchableByEmail :one
-- 检查用户是否允许通过邮箱搜索
SELECT allow_search_by_email 
FROM user_settings 
WHERE user_id = $1;

-- name: ShouldShowOnlineStatus :one
-- 检查用户是否显示在线状态
SELECT show_online_status 
FROM user_settings 
WHERE user_id = $1;

-- name: GetUserNotificationPreferences :one
-- 获取用户通知偏好
SELECT 
    enable_friend_request,
    enable_chatroom_messages,
    enable_system_notice,
    enable_sound,
    enable_desktop_notification
FROM user_settings 
WHERE user_id = $1;

-- name: GetUserPrivacyPreferences :one
-- 获取用户隐私偏好
SELECT 
    allow_search_by_phone,
    allow_search_by_email,
    show_online_status
FROM user_settings 
WHERE user_id = $1;

-- =============================================
-- 3. 批量操作 (Batch Operations)
-- =============================================

-- name: GetUsersWithFriendRequestEnabled :many
-- 获取允许接收好友请求的用户ID列表
SELECT user_id 
FROM user_settings 
WHERE enable_friend_request = true 
    AND user_id = ANY($1::varchar[]);

-- name: GetSearchableUsersByPhone :many
-- 获取允许通过手机号搜索的用户
SELECT u.user_id, u.username, u.nickname, u.avatar_url, u.online_status
FROM users u
JOIN user_settings us ON u.user_id = us.user_id
WHERE us.allow_search_by_phone = true 
    AND u.phone_number = $1 
    AND u.account_status = 'active';

-- name: GetSearchableUsersByEmail :many
-- 获取允许通过邮箱搜索的用户
SELECT u.user_id, u.username, u.nickname, u.avatar_url, u.online_status
FROM users u
JOIN user_settings us ON u.user_id = us.user_id
WHERE us.allow_search_by_email = true 
    AND u.email = $1 
    AND u.account_status = 'active';
