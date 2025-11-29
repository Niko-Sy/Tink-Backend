-- =============================================
-- 用户设置相关SQL查询 (User Settings Queries)
-- 对应API: 用户设置接口
-- 注意: 需要先创建 user_settings 表
-- =============================================

-- =============================================
-- 表结构参考 (需要在migration中创建)
-- =============================================
-- CREATE TABLE user_settings (
--     user_id VARCHAR(10) PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
--     -- 通知设置
--     enable_friend_request BOOLEAN NOT NULL DEFAULT TRUE,
--     enable_chatroom_message BOOLEAN NOT NULL DEFAULT TRUE,
--     enable_system_notice BOOLEAN NOT NULL DEFAULT TRUE,
--     enable_sound BOOLEAN NOT NULL DEFAULT TRUE,
--     enable_desktop_notification BOOLEAN NOT NULL DEFAULT TRUE,
--     -- 隐私设置
--     allow_search_by_phone BOOLEAN NOT NULL DEFAULT TRUE,
--     allow_search_by_email BOOLEAN NOT NULL DEFAULT TRUE,
--     show_online_status BOOLEAN NOT NULL DEFAULT TRUE,
--     -- 其他设置
--     theme VARCHAR(20) DEFAULT 'light',
--     language VARCHAR(10) DEFAULT 'zh-CN',
--     updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
-- );

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
    enable_chatroom_message,
    enable_system_notice,
    enable_sound,
    enable_desktop_notification,
    allow_search_by_phone,
    allow_search_by_email,
    show_online_status,
    theme,
    language,
    updated_at;

-- name: GetUserSettings :one
-- 获取用户设置 GET /users/me/settings
SELECT 
    user_id,
    enable_friend_request,
    enable_chatroom_message,
    enable_system_notice,
    enable_sound,
    enable_desktop_notification,
    allow_search_by_phone,
    allow_search_by_email,
    show_online_status,
    theme,
    language,
    updated_at
FROM user_settings 
WHERE user_id = $1;

-- name: UpdateUserSettings :one
-- 更新用户设置 PUT /users/me/settings
UPDATE user_settings 
SET 
    enable_friend_request = COALESCE($2, enable_friend_request),
    enable_chatroom_message = COALESCE($3, enable_chatroom_message),
    enable_system_notice = COALESCE($4, enable_system_notice),
    enable_sound = COALESCE($5, enable_sound),
    enable_desktop_notification = COALESCE($6, enable_desktop_notification),
    allow_search_by_phone = COALESCE($7, allow_search_by_phone),
    allow_search_by_email = COALESCE($8, allow_search_by_email),
    show_online_status = COALESCE($9, show_online_status),
    theme = COALESCE($10, theme),
    language = COALESCE($11, language),
    updated_at = NOW()
WHERE user_id = $1
RETURNING 
    user_id,
    enable_friend_request,
    enable_chatroom_message,
    enable_system_notice,
    enable_sound,
    enable_desktop_notification,
    allow_search_by_phone,
    allow_search_by_email,
    show_online_status,
    theme,
    language,
    updated_at;

-- name: UpdateNotificationSettings :exec
-- 更新通知设置
UPDATE user_settings 
SET 
    enable_friend_request = COALESCE($2, enable_friend_request),
    enable_chatroom_message = COALESCE($3, enable_chatroom_message),
    enable_system_notice = COALESCE($4, enable_system_notice),
    enable_sound = COALESCE($5, enable_sound),
    enable_desktop_notification = COALESCE($6, enable_desktop_notification),
    updated_at = NOW()
WHERE user_id = $1;

-- name: UpdatePrivacySettings :exec
-- 更新隐私设置
UPDATE user_settings 
SET 
    allow_search_by_phone = COALESCE($2, allow_search_by_phone),
    allow_search_by_email = COALESCE($3, allow_search_by_email),
    show_online_status = COALESCE($4, show_online_status),
    updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateThemeSettings :exec
-- 更新主题设置
UPDATE user_settings 
SET 
    theme = $2,
    updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateLanguageSettings :exec
-- 更新语言设置
UPDATE user_settings 
SET 
    language = $2,
    updated_at = NOW()
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
    enable_chatroom_message,
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
