-- =============================================
-- 聊天室相关SQL查询 (Chatroom Queries)
-- 对应API: 聊天室管理接口 + 聊天室成员管理接口
-- =============================================

-- =============================================
-- 1. 聊天室基础操作 (Chatroom CRUD)
-- =============================================

-- name: CreateChatroom :one
-- 创建聊天室 POST /chatrooms
INSERT INTO chatrooms (
    room_name,
    description,
    icon_url,
    room_type,
    access_password
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING 
    room_id,
    room_name,
    description,
    icon_url,
    room_type,
    access_password,
    member_count,
    online_count,
    room_status,
    created_at,
    last_active_at;

-- name: GetChatroomByID :one
-- 获取聊天室详情 GET /chatrooms/:roomId
SELECT 
    room_id,
    room_name,
    description,
    icon_url,
    room_type,
    access_password,
    member_count,
    online_count,
    room_status,
    created_at,
    last_active_at
FROM chatrooms 
WHERE room_id = $1 AND room_status = 'active';

-- name: GetChatroomWithoutPassword :one
-- 获取聊天室详情（不含密码，用于公开展示）
SELECT 
    room_id,
    room_name,
    description,
    icon_url,
    room_type,
    member_count,
    online_count,
    room_status,
    created_at,
    last_active_at
FROM chatrooms 
WHERE room_id = $1 AND room_status = 'active';

-- name: UpdateChatroom :one
-- 更新聊天室信息 PUT /chatrooms/:roomId
UPDATE chatrooms 
SET 
    room_name = COALESCE($2, room_name),
    description = COALESCE($3, description),
    icon_url = COALESCE($4, icon_url),
    room_type = COALESCE($5, room_type),
    access_password = COALESCE($6, access_password),
    last_active_at = NOW()
WHERE room_id = $1 AND room_status = 'active'
RETURNING 
    room_id,
    room_name,
    description,
    icon_url,
    room_type,
    access_password,
    member_count,
    online_count,
    room_status,
    created_at,
    last_active_at;

-- name: DeleteChatroom :exec
-- 删除聊天室（软删除）DELETE /chatrooms/:roomId
UPDATE chatrooms 
SET room_status = 'deleted'
WHERE room_id = $1;

-- name: ArchiveChatroom :exec
-- 归档聊天室
UPDATE chatrooms 
SET room_status = 'archived'
WHERE room_id = $1;

-- name: VerifyChatroomPassword :one
-- 验证聊天室密码
SELECT EXISTS(
    SELECT 1 FROM chatrooms 
    WHERE room_id = $1 AND access_password = $2 AND room_status = 'active'
) AS is_valid;

-- name: IsChatroomPublic :one
-- 检查聊天室是否为公开
SELECT room_type = 'public' AS is_public
FROM chatrooms 
WHERE room_id = $1 AND room_status = 'active';

-- =============================================
-- 2. 聊天室列表查询 (Chatroom List Queries)
-- =============================================

-- name: ListUserChatrooms :many
-- 获取用户的聊天室列表 GET /users/me/chatrooms
SELECT 
    cr.room_id,
    cr.room_name,
    cr.description,
    cr.icon_url,
    cr.room_type,
    cr.member_count,
    cr.online_count,
    cr.room_status,
    cr.created_at,
    cr.last_active_at,
    cm.member_rel_id,
    cm.joined_at,
    cm.member_role,
    cm.mute_status,
    cm.mute_expires_at,
    cm.last_read_at,
    cm.is_active
FROM chatrooms cr
JOIN chatroom_members cm ON cr.room_id = cm.room_id
WHERE cm.user_id = $1 AND cm.is_active = true AND cr.room_status = 'active'
ORDER BY cr.last_active_at DESC NULLS LAST
LIMIT $2 OFFSET $3;

-- name: CountUserChatrooms :one
-- 统计用户加入的聊天室数量
SELECT COUNT(*) 
FROM chatrooms cr
JOIN chatroom_members cm ON cr.room_id = cm.room_id
WHERE cm.user_id = $1 AND cm.is_active = true AND cr.room_status = 'active';

-- name: ListPublicChatrooms :many
-- 获取公开聊天室列表
SELECT 
    room_id,
    room_name,
    description,
    icon_url,
    room_type,
    member_count,
    online_count,
    created_at,
    last_active_at
FROM chatrooms 
WHERE room_type = 'public' AND room_status = 'active'
ORDER BY online_count DESC, member_count DESC
LIMIT $1 OFFSET $2;

-- name: SearchChatrooms :many
-- 搜索聊天室
SELECT 
    room_id,
    room_name,
    description,
    icon_url,
    room_type,
    member_count,
    online_count,
    created_at,
    last_active_at
FROM chatrooms 
WHERE 
    room_status = 'active'
    AND room_type = 'public'
    AND (
        room_name ILIKE '%' || $1 || '%' 
        OR description ILIKE '%' || $1 || '%'
    )
ORDER BY member_count DESC
LIMIT $2 OFFSET $3;

-- =============================================
-- 3. 聊天室成员操作 (Member Operations)
-- =============================================

-- name: JoinChatroom :one
-- 加入聊天室 POST /chatrooms/:roomId/join
INSERT INTO chatroom_members (
    user_id,
    room_id,
    member_role
) VALUES (
    $1, $2, $3
) 
ON CONFLICT (user_id, room_id) 
DO UPDATE SET 
    is_active = true,
    joined_at = NOW(),
    left_at = NULL,
    member_role = EXCLUDED.member_role
RETURNING 
    member_rel_id,
    user_id,
    room_id,
    joined_at,
    left_at,
    last_read_at,
    member_role,
    mute_status,
    mute_expires_at,
    is_active;

-- name: LeaveChatroom :exec
-- 退出聊天室 POST /chatrooms/:roomId/leave
UPDATE chatroom_members 
SET 
    is_active = false,
    left_at = NOW()
WHERE user_id = $1 AND room_id = $2;

-- name: KickMember :exec
-- 踢出成员 POST /chatrooms/:roomId/members/:userId/kick
UPDATE chatroom_members 
SET 
    is_active = false,
    left_at = NOW()
WHERE user_id = $1 AND room_id = $2;

-- name: GetUserChatroomMembership :one
-- 获取用户在聊天室的成员信息 GET /chatrooms/:roomId/members/:userId
SELECT 
    member_rel_id,
    user_id,
    room_id,
    joined_at,
    left_at,
    last_read_at,
    member_role,
    mute_status,
    mute_expires_at,
    is_active
FROM chatroom_members 
WHERE user_id = $1 AND room_id = $2;

-- name: GetActiveMembership :one
-- 获取有效的成员关系
SELECT 
    member_rel_id,
    user_id,
    room_id,
    joined_at,
    left_at,
    last_read_at,
    member_role,
    mute_status,
    mute_expires_at,
    is_active
FROM chatroom_members 
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: IsUserInChatroom :one
-- 检查用户是否在聊天室中
SELECT EXISTS(
    SELECT 1 FROM chatroom_members 
    WHERE user_id = $1 AND room_id = $2 AND is_active = true
) AS is_member;

-- name: GetMemberByRelID :one
-- 通过关系ID获取成员信息
SELECT 
    member_rel_id,
    user_id,
    room_id,
    joined_at,
    left_at,
    last_read_at,
    member_role,
    mute_status,
    mute_expires_at,
    is_active
FROM chatroom_members 
WHERE member_rel_id = $1;

-- =============================================
-- 4. 成员列表查询 (Member List Queries)
-- =============================================

-- name: GetChatroomMembers :many
-- 获取聊天室成员列表 GET /chatrooms/:roomId/members
SELECT 
    u.user_id,
    u.username,
    u.nickname,
    u.avatar_url,
    u.online_status,
    cm.member_rel_id,
    cm.joined_at,
    cm.member_role,
    cm.mute_status,
    cm.mute_expires_at,
    cm.last_read_at,
    cm.is_active
FROM chatroom_members cm
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1 AND cm.is_active = true
ORDER BY 
    CASE cm.member_role
        WHEN 'owner' THEN 1
        WHEN 'admin' THEN 2
        ELSE 3
    END,
    cm.joined_at ASC
LIMIT $2 OFFSET $3;

-- name: GetOnlineChatroomMembers :many
-- 获取聊天室在线成员列表 GET /chatrooms/:roomId/members?status=online
SELECT 
    u.user_id,
    u.username,
    u.nickname,
    u.avatar_url,
    u.online_status,
    cm.member_rel_id,
    cm.joined_at,
    cm.member_role,
    cm.mute_status,
    cm.mute_expires_at,
    cm.is_active
FROM chatroom_members cm
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1 AND cm.is_active = true AND u.online_status IN ('online', 'away', 'do_not_disturb')
ORDER BY 
    CASE cm.member_role
        WHEN 'owner' THEN 1
        WHEN 'admin' THEN 2
        ELSE 3
    END,
    cm.joined_at ASC
LIMIT $2 OFFSET $3;

-- name: CountChatroomMembers :one
-- 统计聊天室成员数量
SELECT COUNT(*) 
FROM chatroom_members 
WHERE room_id = $1 AND is_active = true;

-- name: CountOnlineChatroomMembers :one
-- 统计聊天室在线成员数量
SELECT COUNT(*) 
FROM chatroom_members cm
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1 AND cm.is_active = true AND u.online_status IN ('online', 'away', 'do_not_disturb');

-- name: SearchChatroomMembers :many
-- 在聊天室内搜索成员（模糊查询用户名或昵称）
SELECT 
    u.user_id,
    u.username,
    u.nickname,
    u.avatar_url,
    u.online_status
FROM chatroom_members cm
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1 
    AND cm.is_active = true
    AND (
        u.username ILIKE '%' || $2 || '%' 
        OR u.nickname ILIKE '%' || $2 || '%'
    )
ORDER BY 
    CASE WHEN u.username ILIKE $2 || '%' THEN 0  -- 前缀匹配优先
         WHEN u.nickname ILIKE $2 || '%' THEN 1
         ELSE 2 
    END,
    u.username ASC
LIMIT $3 OFFSET $4;

-- name: CountSearchChatroomMembers :one
-- 统计搜索结果数量
SELECT COUNT(*) 
FROM chatroom_members cm
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1 
    AND cm.is_active = true
    AND (
        u.username ILIKE '%' || $2 || '%' 
        OR u.nickname ILIKE '%' || $2 || '%'
    );

-- name: GetChatroomOwner :one
-- 获取聊天室房主
SELECT 
    u.user_id,
    u.username,
    u.nickname,
    u.avatar_url,
    u.online_status,
    cm.member_rel_id,
    cm.joined_at,
    cm.member_role
FROM chatroom_members cm
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1 AND cm.member_role = 'owner' AND cm.is_active = true;

-- name: GetChatroomAdmins :many
-- 获取聊天室管理员列表
SELECT 
    u.user_id,
    u.username,
    u.nickname,
    u.avatar_url,
    u.online_status,
    cm.member_rel_id,
    cm.joined_at,
    cm.member_role
FROM chatroom_members cm
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1 AND cm.member_role IN ('owner', 'admin') AND cm.is_active = true
ORDER BY 
    CASE cm.member_role
        WHEN 'owner' THEN 1
        ELSE 2
    END;

-- =============================================
-- 5. 成员角色管理 (Member Role Management)
-- =============================================

-- name: SetMemberRole :exec
-- 设置成员角色 POST /chatrooms/:roomId/members/:userId/set-admin
UPDATE chatroom_members 
SET member_role = $3
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: SetMemberAsAdmin :exec
-- 设置管理员 POST /chatrooms/:roomId/members/:userId/set-admin
UPDATE chatroom_members 
SET member_role = 'admin'
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: RemoveMemberAdmin :exec
-- 取消管理员 POST /chatrooms/:roomId/members/:userId/remove-admin
UPDATE chatroom_members 
SET member_role = 'member'
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: TransferOwnership :exec
-- 转让房主
UPDATE chatroom_members 
SET member_role = CASE 
    WHEN user_id = $1 THEN 'member'
    WHEN user_id = $2 THEN 'owner'
    ELSE member_role
END
WHERE room_id = $3 AND user_id IN ($1, $2) AND is_active = true;

-- name: GetMemberRole :one
-- 获取成员角色
SELECT member_role 
FROM chatroom_members 
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: IsUserOwner :one
-- 检查用户是否为房主
SELECT EXISTS(
    SELECT 1 FROM chatroom_members 
    WHERE user_id = $1 AND room_id = $2 AND member_role = 'owner' AND is_active = true
) AS is_owner;

-- name: IsUserAdminOrOwner :one
-- 检查用户是否为管理员或房主
SELECT EXISTS(
    SELECT 1 FROM chatroom_members 
    WHERE user_id = $1 AND room_id = $2 AND member_role IN ('owner', 'admin') AND is_active = true
) AS is_admin_or_owner;

-- =============================================
-- 6. 禁言管理 (Mute Management)
-- =============================================

-- name: MuteMember :exec
-- 禁言成员 POST /chatrooms/:roomId/members/:userId/mute
UPDATE chatroom_members 
SET 
    mute_status = 'muted',
    mute_expires_at = $3
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: UnmuteMember :exec
-- 解除禁言 POST /chatrooms/:roomId/members/:userId/unmute
UPDATE chatroom_members 
SET 
    mute_status = 'not_muted',
    mute_expires_at = NULL
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: IsMemberMuted :one
-- 检查成员是否被禁言
SELECT 
    CASE 
        WHEN mute_status = 'muted' AND (mute_expires_at IS NULL OR mute_expires_at > NOW()) 
        THEN true 
        ELSE false 
    END as is_muted
FROM chatroom_members 
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: GetMutedMembers :many
-- 获取被禁言的成员列表
SELECT 
    u.user_id,
    u.username,
    u.nickname,
    u.avatar_url,
    cm.member_rel_id,
    cm.mute_status,
    cm.mute_expires_at
FROM chatroom_members cm
JOIN users u ON cm.user_id = u.user_id
WHERE cm.room_id = $1 AND cm.is_active = true AND cm.mute_status = 'muted'
ORDER BY cm.mute_expires_at DESC NULLS FIRST;

-- name: ClearExpiredMutes :exec
-- 清除过期的禁言
UPDATE chatroom_members 
SET 
    mute_status = 'not_muted',
    mute_expires_at = NULL
WHERE mute_status = 'muted' AND mute_expires_at IS NOT NULL AND mute_expires_at <= NOW();

-- =============================================
-- 7. 消息已读管理 (Read Status Management)
-- =============================================

-- name: UpdateMemberLastReadTime :exec
-- 更新最后阅读时间 POST /chatrooms/:roomId/messages/read
UPDATE chatroom_members 
SET last_read_at = NOW()
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: UpdateMemberLastReadToMessage :exec
-- 更新最后阅读到指定消息
UPDATE chatroom_members 
SET last_read_at = (SELECT sent_at FROM messages WHERE message_id = $3)
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: GetMemberLastReadTime :one
-- 获取成员最后阅读时间
SELECT last_read_at 
FROM chatroom_members 
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- =============================================
-- 8. 聊天室统计 (Chatroom Statistics)
-- =============================================

-- name: IncrementChatroomMemberCount :exec
-- 增加成员计数
UPDATE chatrooms 
SET member_count = member_count + 1
WHERE room_id = $1;

-- name: DecrementChatroomMemberCount :exec
-- 减少成员计数
UPDATE chatrooms 
SET member_count = GREATEST(member_count - 1, 0)
WHERE room_id = $1;

-- name: IncrementChatroomOnlineCount :exec
-- 增加在线人数
UPDATE chatrooms 
SET online_count = online_count + 1
WHERE room_id = $1;

-- name: DecrementChatroomOnlineCount :exec
-- 减少在线人数
UPDATE chatrooms 
SET online_count = GREATEST(online_count - 1, 0)
WHERE room_id = $1;

-- name: UpdateChatroomLastActiveTime :exec
-- 更新最后活跃时间
UPDATE chatrooms 
SET last_active_at = NOW()
WHERE room_id = $1;

-- name: SyncChatroomMemberCount :exec
-- 同步成员计数（用于数据修复）
UPDATE chatrooms 
SET member_count = (
    SELECT COUNT(*) FROM chatroom_members 
    WHERE room_id = chatrooms.room_id AND is_active = true
)
WHERE room_id = $1;

-- name: SyncChatroomOnlineCount :exec
-- 同步在线人数（用于数据修复）
UPDATE chatrooms 
SET online_count = (
    SELECT COUNT(*) FROM chatroom_members cm
    JOIN users u ON cm.user_id = u.user_id
    WHERE cm.room_id = chatrooms.room_id AND cm.is_active = true 
    AND u.online_status IN ('online', 'away', 'do_not_disturb')
)
WHERE room_id = $1;