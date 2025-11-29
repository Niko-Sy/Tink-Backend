-- =============================================
-- 消息相关SQL查询 (Message Queries)
-- 对应API: 消息相关接口
-- =============================================

-- =============================================
-- 1. 消息基础操作 (Message CRUD)
-- =============================================

-- name: CreateMessage :one
-- 发送消息 POST /chatrooms/:roomId/messages
INSERT INTO messages (
    content,
    message_type,
    quoted_message_id,
    sender_id,
    room_id
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING 
    message_id,
    sent_at,
    content,
    message_type,
    quoted_message_id,
    sender_id,
    room_id;

-- name: GetMessageByID :one
-- 获取单条消息
SELECT 
    message_id,
    sent_at,
    content,
    message_type,
    quoted_message_id,
    sender_id,
    room_id
FROM messages 
WHERE message_id = $1;

-- name: GetMessageWithSender :one
-- 获取消息及发送者信息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.message_id = $1;

-- name: UpdateMessage :one
-- 编辑消息 PUT /chatrooms/:roomId/messages/:messageId
UPDATE messages 
SET 
    content = $2,
    sent_at = NOW()
WHERE message_id = $1
RETURNING 
    message_id,
    sent_at,
    content,
    message_type,
    quoted_message_id,
    sender_id,
    room_id;

-- name: DeleteMessage :exec
-- 删除消息 DELETE /chatrooms/:roomId/messages/:messageId
DELETE FROM messages 
WHERE message_id = $1;

-- name: DeleteMessageSoft :one
-- 软删除消息（将内容置为系统消息提示）
UPDATE messages 
SET 
    content = '该消息已被删除',
    message_type = 'system_notification'
WHERE message_id = $1
RETURNING 
    message_id,
    sent_at,
    content,
    message_type,
    quoted_message_id,
    sender_id,
    room_id;

-- =============================================
-- 2. 消息列表查询 (Message List Queries)
-- =============================================

-- name: GetMessagesByRoom :many
-- 获取聊天室消息历史 GET /chatrooms/:roomId/messages
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.room_id = $1
ORDER BY m.sent_at DESC
LIMIT $2 OFFSET $3;

-- name: GetMessagesByRoomAsc :many
-- 获取聊天室消息历史（时间正序）
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.room_id = $1
ORDER BY m.sent_at ASC
LIMIT $2 OFFSET $3;

-- name: GetMessagesBefore :many
-- 获取指定消息之前的消息 GET /chatrooms/:roomId/messages?before=M100
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.room_id = $1 
    AND m.sent_at < (SELECT sent_at FROM messages WHERE message_id = $2)
ORDER BY m.sent_at DESC
LIMIT $3;

-- name: GetMessagesAfter :many
-- 获取指定消息之后的消息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.room_id = $1 
    AND m.sent_at > (SELECT sent_at FROM messages WHERE message_id = $2)
ORDER BY m.sent_at ASC
LIMIT $3;

-- name: GetLatestMessages :many
-- 获取最新消息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.room_id = $1
ORDER BY m.sent_at DESC
LIMIT $2;

-- name: GetMessagesByTimeRange :many
-- 获取时间范围内的消息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.room_id = $1 
    AND m.sent_at >= $2 
    AND m.sent_at <= $3
ORDER BY m.sent_at ASC;

-- =============================================
-- 3. 消息统计与未读 (Message Statistics)
-- =============================================

-- name: CountMessagesInRoom :one
-- 统计聊天室消息数量
SELECT COUNT(*) 
FROM messages 
WHERE room_id = $1;

-- name: GetUnreadMessageCount :one
-- 获取未读消息数量
SELECT COUNT(*) 
FROM messages m
LEFT JOIN chatroom_members cm ON m.room_id = cm.room_id AND cm.user_id = $1
WHERE m.room_id = $2 
    AND m.sent_at > COALESCE(cm.last_read_at, '1970-01-01'::TIMESTAMPTZ);

-- name: GetUnreadMessages :many
-- 获取未读消息列表
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
LEFT JOIN chatroom_members cm ON m.room_id = cm.room_id AND cm.user_id = $1
WHERE m.room_id = $2 
    AND m.sent_at > COALESCE(cm.last_read_at, '1970-01-01'::TIMESTAMPTZ)
ORDER BY m.sent_at ASC
LIMIT $3;

-- name: GetLastMessageInRoom :one
-- 获取聊天室最后一条消息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.room_id = $1
ORDER BY m.sent_at DESC
LIMIT 1;

-- name: GetUserUnreadCountsInAllRooms :many
-- 获取用户在所有聊天室的未读消息数
SELECT 
    cm.room_id,
    COUNT(m.message_id) AS unread_count
FROM chatroom_members cm
LEFT JOIN messages m ON m.room_id = cm.room_id 
    AND m.sent_at > COALESCE(cm.last_read_at, '1970-01-01'::TIMESTAMPTZ)
WHERE cm.user_id = $1 AND cm.is_active = true
GROUP BY cm.room_id;

-- =============================================
-- 4. 消息搜索 (Message Search)
-- =============================================

-- name: SearchMessagesInRoom :many
-- 在聊天室中搜索消息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.room_id = $1 
    AND m.content ILIKE '%' || $2 || '%'
ORDER BY m.sent_at DESC
LIMIT $3 OFFSET $4;

-- name: GetMessagesByUser :many
-- 获取用户发送的消息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id
FROM messages m
WHERE m.sender_id = $1
ORDER BY m.sent_at DESC
LIMIT $2 OFFSET $3;

-- name: GetMessagesByUserInRoom :many
-- 获取用户在指定聊天室发送的消息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id
FROM messages m
WHERE m.sender_id = $1 AND m.room_id = $2
ORDER BY m.sent_at DESC
LIMIT $3 OFFSET $4;

-- =============================================
-- 5. 引用消息 (Quoted Messages)
-- =============================================

-- name: GetQuotedMessage :one
-- 获取被引用的消息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.message_id = $1;

-- name: GetMessagesQuotingThis :many
-- 获取引用了指定消息的消息列表
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.quoted_message_id = $1
ORDER BY m.sent_at ASC;

-- =============================================
-- 6. 消息权限验证 (Permission Checks)
-- =============================================

-- name: IsMessageSender :one
-- 检查用户是否是消息发送者
SELECT EXISTS(
    SELECT 1 FROM messages 
    WHERE message_id = $1 AND sender_id = $2
) AS is_sender;

-- name: GetMessageSender :one
-- 获取消息发送者ID
SELECT sender_id 
FROM messages 
WHERE message_id = $1;

-- name: GetMessageRoom :one
-- 获取消息所属聊天室ID
SELECT room_id 
FROM messages 
WHERE message_id = $1;

-- =============================================
-- 7. 批量操作 (Batch Operations)
-- =============================================

-- name: DeleteMessagesByRoom :exec
-- 删除聊天室所有消息
DELETE FROM messages 
WHERE room_id = $1;

-- name: DeleteMessagesByUser :exec
-- 删除用户所有消息
DELETE FROM messages 
WHERE sender_id = $1;

-- name: DeleteMessagesByUserInRoom :exec
-- 删除用户在指定聊天室的所有消息
DELETE FROM messages 
WHERE sender_id = $1 AND room_id = $2;

-- name: GetMessagesByIDs :many
-- 批量获取消息
SELECT 
    m.message_id,
    m.sent_at,
    m.content,
    m.message_type,
    m.quoted_message_id,
    m.sender_id,
    m.room_id,
    u.username,
    u.nickname,
    u.avatar_url
FROM messages m
LEFT JOIN users u ON m.sender_id = u.user_id
WHERE m.message_id = ANY($1::varchar[])
ORDER BY m.sent_at ASC;