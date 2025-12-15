-- =============================================
-- 好友关系相关SQL查询 (Friend Queries)
-- 对应API: 好友关系接口
-- 注意: 对应 migration/000002_first_newfunction.up.sql 中的表结构
-- =============================================

-- =============================================
-- 表结构说明 (已在migration中创建)
-- =============================================
-- friend_request: 好友请求表
--   - request_id (varchar 12, PK, 格式: FR0000000001)
--   - from_user_id (varchar 10, FK -> users)
--   - to_user_id (varchar 10, FK -> users)
--   - message (TEXT)
--   - status (friend_request_status ENUM: pending/accepted/rejected)
--   - createtime (TIMESTAMPTZ)
--   - handled_at (TIMESTAMPTZ)
-- 
-- friends: 好友关系表 (使用规范化设计，user_id_1 < user_id_2)
--   - relation_id (varchar 11, PK, 格式: R0000000001)
--   - user_id_1 (varchar 10, FK -> users)
--   - user_id_2 (varchar 10, FK -> users)
--   - established_at (TIMESTAMPTZ)
--   - CHECK (user_id_1 < user_id_2)

-- =============================================
-- 1. 好友请求操作 (Friend Request Operations)
-- =============================================

-- name: CreateFriendRequest :one
-- 发送好友请求 POST /friends/request
INSERT INTO friend_request (
    from_user_id,
    to_user_id,
    message
) VALUES (
    $1, $2, $3
) RETURNING 
    request_id,
    from_user_id,
    to_user_id,
    message,
    status,
    createtime,
    handled_at;

-- name: GetFriendRequestByID :one
-- 获取好友请求详情
SELECT 
    request_id,
    from_user_id,
    to_user_id,
    message,
    status,
    createtime,
    handled_at
FROM friend_request 
WHERE request_id = $1;

-- name: GetPendingRequestBetweenUsers :one
-- 检查两个用户之间是否有待处理的请求
SELECT 
    request_id,
    from_user_id,
    to_user_id,
    message,
    status,
    createtime,
    handled_at
FROM friend_request 
WHERE ((from_user_id = $1 AND to_user_id = $2) OR (from_user_id = $2 AND to_user_id = $1))
    AND status = 'pending';

-- name: AcceptFriendRequest :one
-- 接受好友请求 POST /friends/request/:requestId/handle
UPDATE friend_request 
SET 
    status = 'accepted',
    handled_at = NOW()
WHERE request_id = $1 AND to_user_id = $2 AND status = 'pending'
RETURNING 
    request_id,
    from_user_id,
    to_user_id,
    message,
    status,
    createtime,
    handled_at;

-- name: RejectFriendRequest :one
-- 拒绝好友请求 POST /friends/request/:requestId/handle
UPDATE friend_request 
SET 
    status = 'rejected',
    handled_at = NOW()
WHERE request_id = $1 AND to_user_id = $2 AND status = 'pending'
RETURNING 
    request_id,
    from_user_id,
    to_user_id,
    message,
    status,
    createtime,
    handled_at;

-- name: CancelFriendRequest :exec
-- 取消好友请求
DELETE FROM friend_request 
WHERE request_id = $1 AND from_user_id = $2 AND status = 'pending';

-- =============================================
-- 2. 好友请求列表 (Friend Request Lists)
-- =============================================

-- name: GetReceivedFriendRequests :many
-- 获取收到的好友请求 GET /users/me/friend-requests?type=received
SELECT 
    fr.request_id,
    fr.from_user_id,
    fr.to_user_id,
    fr.message,
    fr.status,
    fr.createtime,
    fr.handled_at,
    u.username AS sender_username,
    u.nickname AS sender_nickname,
    u.avatar_url AS sender_avatar,
    u.online_status AS sender_online_status
FROM friend_request fr
JOIN users u ON fr.from_user_id = u.user_id
WHERE fr.to_user_id = $1
ORDER BY fr.createtime DESC
LIMIT $2 OFFSET $3;

-- name: GetSentFriendRequests :many
-- 获取发送的好友请求 GET /users/me/friend-requests?type=sent
SELECT 
    fr.request_id,
    fr.from_user_id,
    fr.to_user_id,
    fr.message,
    fr.status,
    fr.createtime,
    fr.handled_at,
    u.username AS receiver_username,
    u.nickname AS receiver_nickname,
    u.avatar_url AS receiver_avatar,
    u.online_status AS receiver_online_status
FROM friend_request fr
JOIN users u ON fr.to_user_id = u.user_id
WHERE fr.from_user_id = $1
ORDER BY fr.createtime DESC
LIMIT $2 OFFSET $3;

-- name: GetPendingReceivedRequests :many
-- 获取待处理的收到的好友请求
SELECT 
    fr.request_id,
    fr.from_user_id,
    fr.to_user_id,
    fr.message,
    fr.status,
    fr.createtime,
    fr.handled_at,
    u.username AS sender_username,
    u.nickname AS sender_nickname,
    u.avatar_url AS sender_avatar,
    u.online_status AS sender_online_status
FROM friend_request fr
JOIN users u ON fr.from_user_id = u.user_id
WHERE fr.to_user_id = $1 AND fr.status = 'pending'
ORDER BY fr.createtime DESC
LIMIT $2 OFFSET $3;

-- name: CountPendingReceivedRequests :one
-- 统计待处理的收到的好友请求数量
SELECT COUNT(*) 
FROM friend_request 
WHERE to_user_id = $1 AND status = 'pending';

-- name: CountReceivedFriendRequests :one
-- 统计收到的好友请求总数（按状态）
SELECT COUNT(*) 
FROM friend_request 
WHERE to_user_id = $1 AND ($2::text IS NULL OR status = $2::friend_request_status);

-- name: CountSentFriendRequests :one
-- 统计发送的好友请求总数（按状态）
SELECT COUNT(*) 
FROM friend_request 
WHERE from_user_id = $1 AND ($2::text IS NULL OR status = $2::friend_request_status);

-- =============================================
-- 3. 好友关系操作 (Friendship Operations)
-- =============================================

-- name: CreateFriendship :exec
-- 创建好友关系（单条记录，user_id_1 < user_id_2）
-- 调用时需要在应用层确保 user_id_1 < user_id_2
INSERT INTO friends (user_id_1, user_id_2) 
VALUES (
    CASE WHEN $1 < $2 THEN $1 ELSE $2 END,
    CASE WHEN $1 < $2 THEN $2 ELSE $1 END
)
ON CONFLICT (user_id_1, user_id_2) DO NOTHING;

-- name: DeleteFriendship :exec
-- 删除好友关系 DELETE /friends/:userId
DELETE FROM friends 
WHERE (user_id_1 = CASE WHEN $1 < $2 THEN $1 ELSE $2 END 
   AND user_id_2 = CASE WHEN $1 < $2 THEN $2 ELSE $1 END);

-- name: IsFriend :one
-- 检查是否是好友
SELECT EXISTS(
    SELECT 1 FROM friends 
    WHERE (user_id_1 = CASE WHEN $1 < $2 THEN $1 ELSE $2 END 
       AND user_id_2 = CASE WHEN $1 < $2 THEN $2 ELSE $1 END)
) AS is_friend;

-- =============================================
-- 4. 好友列表查询 (Friend List Queries)
-- =============================================

-- name: GetFriends :many
-- 获取好友列表 GET /users/me/friends
SELECT 
    CASE WHEN f.user_id_1 = $1 THEN f.user_id_2 ELSE f.user_id_1 END AS friend_id,
    f.established_at AS friend_since,
    u.username,
    u.nickname,
    u.avatar_url,
    u.bio,
    u.online_status
FROM friends f
JOIN users u ON (CASE WHEN f.user_id_1 = $1 THEN f.user_id_2 ELSE f.user_id_1 END) = u.user_id
WHERE (f.user_id_1 = $1 OR f.user_id_2 = $1) 
    AND u.account_status = 'active'
ORDER BY u.online_status DESC, f.established_at DESC
LIMIT $2 OFFSET $3;

-- name: GetOnlineFriends :many
-- 获取在线好友列表 GET /users/me/friends?status=online
SELECT 
    CASE WHEN f.user_id_1 = $1 THEN f.user_id_2 ELSE f.user_id_1 END AS friend_id,
    f.established_at AS friend_since,
    u.username,
    u.nickname,
    u.avatar_url,
    u.bio,
    u.online_status
FROM friends f
JOIN users u ON (CASE WHEN f.user_id_1 = $1 THEN f.user_id_2 ELSE f.user_id_1 END) = u.user_id
WHERE (f.user_id_1 = $1 OR f.user_id_2 = $1)
    AND u.account_status = 'active'
    AND u.online_status IN ('online', 'away', 'do_not_disturb')
ORDER BY f.established_at DESC
LIMIT $2 OFFSET $3;

-- name: CountFriends :one
-- 统计好友数量
SELECT COUNT(*) 
FROM friends f
JOIN users u ON (CASE WHEN f.user_id_1 = $1 THEN f.user_id_2 ELSE f.user_id_1 END) = u.user_id
WHERE (f.user_id_1 = $1 OR f.user_id_2 = $1) 
    AND u.account_status = 'active';

-- name: CountOnlineFriends :one
-- 统计在线好友数量
SELECT COUNT(*) 
FROM friends f
JOIN users u ON (CASE WHEN f.user_id_1 = $1 THEN f.user_id_2 ELSE f.user_id_1 END) = u.user_id
WHERE (f.user_id_1 = $1 OR f.user_id_2 = $1)
    AND u.account_status = 'active'
    AND u.online_status IN ('online', 'away', 'do_not_disturb');

-- name: SearchFriends :many
-- 搜索好友
SELECT 
    CASE WHEN f.user_id_1 = $1 THEN f.user_id_2 ELSE f.user_id_1 END AS friend_id,
    f.established_at AS friend_since,
    u.username,
    u.nickname,
    u.avatar_url,
    u.bio,
    u.online_status
FROM friends f
JOIN users u ON (CASE WHEN f.user_id_1 = $1 THEN f.user_id_2 ELSE f.user_id_1 END) = u.user_id
WHERE (f.user_id_1 = $1 OR f.user_id_2 = $1)
    AND u.account_status = 'active'
    AND (
        u.username ILIKE '%' || $2 || '%' 
        OR u.nickname ILIKE '%' || $2 || '%'
    )
ORDER BY u.online_status DESC, f.established_at DESC
LIMIT $3 OFFSET $4;

-- name: GetMutualFriends :many
-- 获取共同好友（两个用户的共同好友）
SELECT 
    u.user_id,
    u.username,
    u.nickname,
    u.avatar_url,
    u.online_status
FROM friends f1
JOIN friends f2 ON (
    (f1.user_id_1 = f2.user_id_1 AND f1.user_id_1 != $1 AND f1.user_id_1 != $2)
    OR (f1.user_id_1 = f2.user_id_2 AND f1.user_id_1 != $1 AND f1.user_id_1 != $2)
    OR (f1.user_id_2 = f2.user_id_1 AND f1.user_id_2 != $1 AND f1.user_id_2 != $2)
    OR (f1.user_id_2 = f2.user_id_2 AND f1.user_id_2 != $1 AND f1.user_id_2 != $2)
)
JOIN users u ON u.user_id = COALESCE(
    NULLIF(f1.user_id_1, $1),
    NULLIF(f1.user_id_2, $1)
)
WHERE ((f1.user_id_1 = $1 OR f1.user_id_2 = $1))
    AND ((f2.user_id_1 = $2 OR f2.user_id_2 = $2))
    AND u.account_status = 'active'
ORDER BY u.online_status DESC;
