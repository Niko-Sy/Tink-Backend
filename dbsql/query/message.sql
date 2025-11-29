-- name: CreateMessage :one
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

-- name: GetMessagesByRoom :many
SELECT 
    message_id,
    sent_at,
    content,
    message_type,
    quoted_message_id,
    sender_id,
    room_id
FROM messages 
WHERE room_id = $1
ORDER BY sent_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateMessage :one
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
DELETE FROM messages 
WHERE message_id = $1;

-- name: GetUnreadMessageCount :one
SELECT COUNT(*) 
FROM messages m
LEFT JOIN chatroom_members cm ON m.room_id = cm.room_id AND cm.user_id = $1
WHERE m.room_id = $2 
    AND m.sent_at > COALESCE(cm.last_read_at, '1970-01-01'::TIMESTAMPTZ);