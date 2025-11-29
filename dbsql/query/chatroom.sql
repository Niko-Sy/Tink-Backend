-- name: CreateChatroom :one
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

-- name: UpdateChatroom :one
UPDATE chatrooms 
SET 
    room_name = $2,
    description = $3,
    icon_url = $4,
    room_type = $5,
    access_password = $6,
    last_active_at = NOW()
WHERE room_id = $1
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
UPDATE chatrooms 
SET room_status = 'deleted'
WHERE room_id = $1;

-- name: ListUserChatrooms :many
SELECT 
    cr.room_id,
    cr.room_name,
    cr.description,
    cr.icon_url,
    cr.room_type,
    cr.access_password,
    cr.member_count,
    cr.online_count,
    cr.room_status,
    cr.created_at,
    cr.last_active_at,
    cm.member_rel_id,
    cm.joined_at,
    cm.member_role,
    cm.mute_status,
    cm.is_active
FROM chatrooms cr
JOIN chatroom_members cm ON cr.room_id = cm.room_id
WHERE cm.user_id = $1 AND cm.is_active = true AND cr.room_status = 'active'
ORDER BY cr.last_active_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUserChatrooms :one
SELECT COUNT(*) 
FROM chatrooms cr
JOIN chatroom_members cm ON cr.room_id = cm.room_id
WHERE cm.user_id = $1 AND cm.is_active = true AND cr.room_status = 'active';

-- name: JoinChatroom :one
INSERT INTO chatroom_members (
    user_id,
    room_id,
    member_role
) VALUES (
    $1, $2, 'member'
) 
ON CONFLICT (user_id, room_id) 
DO UPDATE SET 
    is_active = true,
    joined_at = NOW(),
    left_at = NULL
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
UPDATE chatroom_members 
SET 
    is_active = false,
    left_at = NOW()
WHERE user_id = $1 AND room_id = $2;

-- name: GetUserChatroomMembership :one
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

-- name: UpdateMemberLastReadTime :exec
UPDATE chatroom_members 
SET last_read_at = NOW()
WHERE user_id = $1 AND room_id = $2;

-- name: GetChatroomMembers :many
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
WHERE cm.room_id = $1 AND cm.is_active = true
ORDER BY cm.joined_at ASC
LIMIT $2 OFFSET $3;

-- name: CountChatroomMembers :one
SELECT COUNT(*) 
FROM chatroom_members 
WHERE room_id = $1 AND is_active = true;

-- name: SetMemberRole :exec
UPDATE chatroom_members 
SET member_role = $3
WHERE user_id = $1 AND room_id = $2;

-- name: MuteMember :exec
UPDATE chatroom_members 
SET 
    mute_status = 'muted',
    mute_expires_at = $3
WHERE user_id = $1 AND room_id = $2;

-- name: UnmuteMember :exec
UPDATE chatroom_members 
SET 
    mute_status = 'not_muted',
    mute_expires_at = NULL
WHERE user_id = $1 AND room_id = $2;

-- name: IsMemberMuted :one
SELECT 
    CASE 
        WHEN mute_status = 'muted' AND (mute_expires_at IS NULL OR mute_expires_at > NOW()) 
        THEN true 
        ELSE false 
    END as is_muted
FROM chatroom_members 
WHERE user_id = $1 AND room_id = $2 AND is_active = true;

-- name: IncrementChatroomMemberCount :exec
UPDATE chatrooms 
SET member_count = member_count + 1
WHERE room_id = $1;

-- name: DecrementChatroomMemberCount :exec
UPDATE chatrooms 
SET member_count = member_count - 1
WHERE room_id = $1;

-- name: IncrementChatroomOnlineCount :exec
UPDATE chatrooms 
SET online_count = online_count + 1
WHERE room_id = $1;

-- name: DecrementChatroomOnlineCount :exec
UPDATE chatrooms 
SET online_count = online_count - 1
WHERE room_id = $1;

-- name: UpdateChatroomLastActiveTime :exec
UPDATE chatrooms 
SET last_active_at = NOW()
WHERE room_id = $1;