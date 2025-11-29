-- =============================================
-- 好友关系相关SQL查询 (Friend Queries)
-- 对应API: 好友关系接口
-- 注意: 需要先创建 friends 和 friend_requests 表
-- =============================================

-- =============================================
-- 表结构参考 (需要在migration中创建)
-- =============================================
-- CREATE TABLE friend_requests (
--     request_id VARCHAR(10) PRIMARY KEY,
--     sender_id VARCHAR(10) NOT NULL REFERENCES users(user_id),
--     receiver_id VARCHAR(10) NOT NULL REFERENCES users(user_id),
--     message TEXT,
--     status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, accepted, rejected
--     created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     handled_at TIMESTAMPTZ,
--     CONSTRAINT unique_friend_request UNIQUE (sender_id, receiver_id)
-- );
-- 
-- CREATE TABLE friends (
--     id SERIAL PRIMARY KEY,
--     user_id VARCHAR(10) NOT NULL REFERENCES users(user_id),
--     friend_id VARCHAR(10) NOT NULL REFERENCES users(user_id),
--     friend_since TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     CONSTRAINT unique_friendship UNIQUE (user_id, friend_id)
-- );

-- =============================================
-- 1. 好友请求操作 (Friend Request Operations)
-- =============================================

-- name: CreateFriendRequest :one
-- 发送好友请求 POST /friends/request
INSERT INTO friend_requests (
    sender_id,
    receiver_id,
    message
) VALUES (
    $1, $2, $3
) RETURNING 
    request_id,
    sender_id,
    receiver_id,
    message,
    status,
    created_at,
    handled_at;

-- name: GetFriendRequestByID :one
-- 获取好友请求详情
SELECT 
    request_id,
    sender_id,
    receiver_id,
    message,
    status,
    created_at,
    handled_at
FROM friend_requests 
WHERE request_id = $1;

-- name: GetPendingRequestBetweenUsers :one
-- 检查两个用户之间是否有待处理的请求
SELECT 
    request_id,
    sender_id,
    receiver_id,
    message,
    status,
    created_at,
    handled_at
FROM friend_requests 
WHERE ((sender_id = $1 AND receiver_id = $2) OR (sender_id = $2 AND receiver_id = $1))
    AND status = 'pending';

-- name: AcceptFriendRequest :one
-- 接受好友请求 POST /friends/request/:requestId/handle
UPDATE friend_requests 
SET 
    status = 'accepted',
    handled_at = NOW()
WHERE request_id = $1 AND receiver_id = $2 AND status = 'pending'
RETURNING 
    request_id,
    sender_id,
    receiver_id,
    message,
    status,
    created_at,
    handled_at;

-- name: RejectFriendRequest :one
-- 拒绝好友请求 POST /friends/request/:requestId/handle
UPDATE friend_requests 
SET 
    status = 'rejected',
    handled_at = NOW()
WHERE request_id = $1 AND receiver_id = $2 AND status = 'pending'
RETURNING 
    request_id,
    sender_id,
    receiver_id,
    message,
    status,
    created_at,
    handled_at;

-- name: CancelFriendRequest :exec
-- 取消好友请求
DELETE FROM friend_requests 
WHERE request_id = $1 AND sender_id = $2 AND status = 'pending';

-- =============================================
-- 2. 好友请求列表 (Friend Request Lists)
-- =============================================

-- name: GetReceivedFriendRequests :many
-- 获取收到的好友请求 GET /users/me/friend-requests?type=received
SELECT 
    fr.request_id,
    fr.sender_id,
    fr.receiver_id,
    fr.message,
    fr.status,
    fr.created_at,
    fr.handled_at,
    u.username AS sender_username,
    u.nickname AS sender_nickname,
    u.avatar_url AS sender_avatar,
    u.online_status AS sender_online_status
FROM friend_requests fr
JOIN users u ON fr.sender_id = u.user_id
WHERE fr.receiver_id = $1
ORDER BY fr.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetSentFriendRequests :many
-- 获取发送的好友请求 GET /users/me/friend-requests?type=sent
SELECT 
    fr.request_id,
    fr.sender_id,
    fr.receiver_id,
    fr.message,
    fr.status,
    fr.created_at,
    fr.handled_at,
    u.username AS receiver_username,
    u.nickname AS receiver_nickname,
    u.avatar_url AS receiver_avatar,
    u.online_status AS receiver_online_status
FROM friend_requests fr
JOIN users u ON fr.receiver_id = u.user_id
WHERE fr.sender_id = $1
ORDER BY fr.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPendingReceivedRequests :many
-- 获取待处理的收到的好友请求
SELECT 
    fr.request_id,
    fr.sender_id,
    fr.receiver_id,
    fr.message,
    fr.status,
    fr.created_at,
    fr.handled_at,
    u.username AS sender_username,
    u.nickname AS sender_nickname,
    u.avatar_url AS sender_avatar,
    u.online_status AS sender_online_status
FROM friend_requests fr
JOIN users u ON fr.sender_id = u.user_id
WHERE fr.receiver_id = $1 AND fr.status = 'pending'
ORDER BY fr.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPendingReceivedRequests :one
-- 统计待处理的收到的好友请求数量
SELECT COUNT(*) 
FROM friend_requests 
WHERE receiver_id = $1 AND status = 'pending';

-- =============================================
-- 3. 好友关系操作 (Friendship Operations)
-- =============================================

-- name: CreateFriendship :exec
-- 创建好友关系（双向）
INSERT INTO friends (user_id, friend_id) 
VALUES ($1, $2), ($2, $1)
ON CONFLICT (user_id, friend_id) DO NOTHING;

-- name: DeleteFriendship :exec
-- 删除好友关系 DELETE /friends/:userId
DELETE FROM friends 
WHERE (user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1);

-- name: IsFriend :one
-- 检查是否是好友
SELECT EXISTS(
    SELECT 1 FROM friends 
    WHERE user_id = $1 AND friend_id = $2
) AS is_friend;

-- =============================================
-- 4. 好友列表查询 (Friend List Queries)
-- =============================================

-- name: GetFriends :many
-- 获取好友列表 GET /users/me/friends
SELECT 
    f.friend_id,
    f.friend_since,
    u.username,
    u.nickname,
    u.avatar_url,
    u.bio,
    u.online_status
FROM friends f
JOIN users u ON f.friend_id = u.user_id
WHERE f.user_id = $1 AND u.account_status = 'active'
ORDER BY u.online_status DESC, f.friend_since DESC
LIMIT $2 OFFSET $3;

-- name: GetOnlineFriends :many
-- 获取在线好友列表 GET /users/me/friends?status=online
SELECT 
    f.friend_id,
    f.friend_since,
    u.username,
    u.nickname,
    u.avatar_url,
    u.bio,
    u.online_status
FROM friends f
JOIN users u ON f.friend_id = u.user_id
WHERE f.user_id = $1 
    AND u.account_status = 'active'
    AND u.online_status IN ('online', 'away', 'do_not_disturb')
ORDER BY f.friend_since DESC
LIMIT $2 OFFSET $3;

-- name: CountFriends :one
-- 统计好友数量
SELECT COUNT(*) 
FROM friends f
JOIN users u ON f.friend_id = u.user_id
WHERE f.user_id = $1 AND u.account_status = 'active';

-- name: CountOnlineFriends :one
-- 统计在线好友数量
SELECT COUNT(*) 
FROM friends f
JOIN users u ON f.friend_id = u.user_id
WHERE f.user_id = $1 
    AND u.account_status = 'active'
    AND u.online_status IN ('online', 'away', 'do_not_disturb');

-- name: SearchFriends :many
-- 搜索好友
SELECT 
    f.friend_id,
    f.friend_since,
    u.username,
    u.nickname,
    u.avatar_url,
    u.bio,
    u.online_status
FROM friends f
JOIN users u ON f.friend_id = u.user_id
WHERE f.user_id = $1 
    AND u.account_status = 'active'
    AND (
        u.username ILIKE '%' || $2 || '%' 
        OR u.nickname ILIKE '%' || $2 || '%'
    )
ORDER BY u.online_status DESC, f.friend_since DESC
LIMIT $3 OFFSET $4;

-- name: GetMutualFriends :many
-- 获取共同好友
SELECT 
    u.user_id,
    u.username,
    u.nickname,
    u.avatar_url,
    u.online_status
FROM friends f1
JOIN friends f2 ON f1.friend_id = f2.friend_id
JOIN users u ON f1.friend_id = u.user_id
WHERE f1.user_id = $1 AND f2.user_id = $2 AND u.account_status = 'active'
ORDER BY u.online_status DESC;
