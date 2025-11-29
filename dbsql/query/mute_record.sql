-- =============================================
-- 禁言记录相关SQL查询 (Mute Record Queries)
-- 对应API: 聊天室成员管理接口 - 禁言功能
-- =============================================

-- =============================================
-- 1. 聊天室禁言记录 (Room Mute Records)
-- =============================================

-- name: CreateMuteRecord :one
-- 创建禁言记录 POST /chatrooms/:roomId/members/:userId/mute
INSERT INTO mute_records (
    member_rel_id,
    expires_at,
    reason,
    admin_id
) VALUES (
    $1, $2, $3, $4
) RETURNING 
    mute_record_id,
    member_rel_id,
    start_at,
    expires_at,
    reason,
    is_active,
    admin_id;

-- name: GetMuteRecordByID :one
-- 获取禁言记录
SELECT 
    mute_record_id,
    member_rel_id,
    start_at,
    expires_at,
    reason,
    is_active,
    admin_id
FROM mute_records 
WHERE mute_record_id = $1;

-- name: GetActiveMuteRecord :one
-- 获取成员当前有效的禁言记录
SELECT 
    mute_record_id,
    member_rel_id,
    start_at,
    expires_at,
    reason,
    is_active,
    admin_id
FROM mute_records 
WHERE member_rel_id = $1 
    AND is_active = true 
    AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY start_at DESC
LIMIT 1;

-- name: GetMuteRecordsByMember :many
-- 获取成员的所有禁言记录
SELECT 
    mute_record_id,
    member_rel_id,
    start_at,
    expires_at,
    reason,
    is_active,
    admin_id
FROM mute_records 
WHERE member_rel_id = $1
ORDER BY start_at DESC
LIMIT $2 OFFSET $3;

-- name: GetMuteRecordsByRoom :many
-- 获取聊天室的所有禁言记录
SELECT 
    mr.mute_record_id,
    mr.member_rel_id,
    mr.start_at,
    mr.expires_at,
    mr.reason,
    mr.is_active,
    mr.admin_id,
    cm.user_id,
    cm.room_id,
    u.username,
    u.nickname
FROM mute_records mr
JOIN chatroom_members cm ON mr.member_rel_id = cm.member_rel_id
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1
ORDER BY mr.start_at DESC
LIMIT $2 OFFSET $3;

-- name: GetActiveMuteRecordsByRoom :many
-- 获取聊天室当前有效的禁言记录
SELECT 
    mr.mute_record_id,
    mr.member_rel_id,
    mr.start_at,
    mr.expires_at,
    mr.reason,
    mr.is_active,
    mr.admin_id,
    cm.user_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM mute_records mr
JOIN chatroom_members cm ON mr.member_rel_id = cm.member_rel_id
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1 
    AND mr.is_active = true 
    AND (mr.expires_at IS NULL OR mr.expires_at > NOW())
ORDER BY mr.expires_at ASC NULLS LAST;

-- name: DeactivateMuteRecord :exec
-- 解除禁言（使禁言记录失效）POST /chatrooms/:roomId/members/:userId/unmute
UPDATE mute_records 
SET is_active = false
WHERE member_rel_id = $1 AND is_active = true;

-- name: DeactivateMuteRecordByID :exec
-- 通过ID解除禁言
UPDATE mute_records 
SET is_active = false
WHERE mute_record_id = $1;

-- name: ExpireMuteRecords :exec
-- 批量过期禁言记录
UPDATE mute_records 
SET is_active = false
WHERE is_active = true AND expires_at IS NOT NULL AND expires_at <= NOW();

-- name: IsMemberMutedInRoom :one
-- 检查成员在聊天室是否被禁言
SELECT EXISTS(
    SELECT 1 FROM mute_records mr
    JOIN chatroom_members cm ON mr.member_rel_id = cm.member_rel_id
    WHERE cm.user_id = $1 
        AND cm.room_id = $2 
        AND mr.is_active = true 
        AND (mr.expires_at IS NULL OR mr.expires_at > NOW())
) AS is_muted;

-- name: GetMemberMuteExpireTime :one
-- 获取成员禁言到期时间
SELECT mr.expires_at
FROM mute_records mr
JOIN chatroom_members cm ON mr.member_rel_id = cm.member_rel_id
WHERE cm.user_id = $1 
    AND cm.room_id = $2 
    AND mr.is_active = true 
    AND (mr.expires_at IS NULL OR mr.expires_at > NOW())
ORDER BY mr.expires_at DESC NULLS FIRST
LIMIT 1;

-- =============================================
-- 2. 全局禁言记录 (Global Mute Records)
-- =============================================

-- name: CreateGlobalMuteRecord :one
-- 创建全局禁言记录（超级管理员操作）
INSERT INTO global_mute_records (
    muted_user_id,
    expires_at,
    reason,
    admin_id
) VALUES (
    $1, $2, $3, $4
) RETURNING 
    global_mute_id,
    muted_user_id,
    start_at,
    expires_at,
    reason,
    is_active,
    admin_id;

-- name: GetGlobalMuteRecordByID :one
-- 获取全局禁言记录
SELECT 
    global_mute_id,
    muted_user_id,
    start_at,
    expires_at,
    reason,
    is_active,
    admin_id
FROM global_mute_records 
WHERE global_mute_id = $1;

-- name: GetActiveGlobalMuteRecord :one
-- 获取用户当前有效的全局禁言记录
SELECT 
    global_mute_id,
    muted_user_id,
    start_at,
    expires_at,
    reason,
    is_active,
    admin_id
FROM global_mute_records 
WHERE muted_user_id = $1 
    AND is_active = true 
    AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY start_at DESC
LIMIT 1;

-- name: GetGlobalMuteRecordsByUser :many
-- 获取用户的所有全局禁言记录
SELECT 
    global_mute_id,
    muted_user_id,
    start_at,
    expires_at,
    reason,
    is_active,
    admin_id
FROM global_mute_records 
WHERE muted_user_id = $1
ORDER BY start_at DESC
LIMIT $2 OFFSET $3;

-- name: GetAllActiveGlobalMuteRecords :many
-- 获取所有有效的全局禁言记录
SELECT 
    gmr.global_mute_id,
    gmr.muted_user_id,
    gmr.start_at,
    gmr.expires_at,
    gmr.reason,
    gmr.is_active,
    gmr.admin_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM global_mute_records gmr
JOIN users u ON gmr.muted_user_id = u.user_id
WHERE gmr.is_active = true 
    AND (gmr.expires_at IS NULL OR gmr.expires_at > NOW())
ORDER BY gmr.expires_at ASC NULLS LAST
LIMIT $1 OFFSET $2;

-- name: DeactivateGlobalMuteRecord :exec
-- 解除全局禁言
UPDATE global_mute_records 
SET is_active = false
WHERE muted_user_id = $1 AND is_active = true;

-- name: DeactivateGlobalMuteRecordByID :exec
-- 通过ID解除全局禁言
UPDATE global_mute_records 
SET is_active = false
WHERE global_mute_id = $1;

-- name: ExpireGlobalMuteRecords :exec
-- 批量过期全局禁言记录
UPDATE global_mute_records 
SET is_active = false
WHERE is_active = true AND expires_at IS NOT NULL AND expires_at <= NOW();

-- name: IsUserGloballyMuted :one
-- 检查用户是否被全局禁言
SELECT EXISTS(
    SELECT 1 FROM global_mute_records 
    WHERE muted_user_id = $1 
        AND is_active = true 
        AND (expires_at IS NULL OR expires_at > NOW())
) AS is_globally_muted;

-- name: GetUserGlobalMuteExpireTime :one
-- 获取用户全局禁言到期时间
SELECT expires_at
FROM global_mute_records 
WHERE muted_user_id = $1 
    AND is_active = true 
    AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY expires_at DESC NULLS FIRST
LIMIT 1;

-- =============================================
-- 3. 综合禁言检查 (Combined Mute Checks)
-- =============================================

-- name: CanUserSendMessageInRoom :one
-- 检查用户是否可以在聊天室发送消息（综合检查全局禁言和聊天室禁言）
SELECT 
    NOT EXISTS(
        SELECT 1 FROM global_mute_records 
        WHERE muted_user_id = $1 
            AND is_active = true 
            AND (expires_at IS NULL OR expires_at > NOW())
    )
    AND NOT EXISTS(
        SELECT 1 FROM mute_records mr
        JOIN chatroom_members cm ON mr.member_rel_id = cm.member_rel_id
        WHERE cm.user_id = $1 
            AND cm.room_id = $2 
            AND mr.is_active = true 
            AND (mr.expires_at IS NULL OR mr.expires_at > NOW())
    ) AS can_send;

-- name: GetUserMuteStatus :one
-- 获取用户的禁言状态（返回全局禁言和聊天室禁言状态）
SELECT 
    EXISTS(
        SELECT 1 FROM global_mute_records 
        WHERE muted_user_id = $1 
            AND is_active = true 
            AND (expires_at IS NULL OR expires_at > NOW())
    ) AS is_globally_muted,
    EXISTS(
        SELECT 1 FROM mute_records mr
        JOIN chatroom_members cm ON mr.member_rel_id = cm.member_rel_id
        WHERE cm.user_id = $1 
            AND cm.room_id = $2 
            AND mr.is_active = true 
            AND (mr.expires_at IS NULL OR mr.expires_at > NOW())
    ) AS is_room_muted;
