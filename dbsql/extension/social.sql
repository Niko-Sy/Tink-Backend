-- name: SocialProfile :one
SELECT u.user_id,u.username,COALESCE(u.nickname,u.username)::text AS nickname,
 COALESCE(u.avatar_url,'')::text AS avatar,COALESCE(u.bio,'')::text AS signature,
 COALESCE(u.account_status::text,'active')::text AS account_status,
 COALESCE(u.online_status::text,'offline')::text AS online_status,
 COALESCE(s.show_online_status,true)::boolean AS show_online_status,
 COALESCE(s.enable_friend_request,true)::boolean AS enable_friend_request
FROM users u LEFT JOIN user_settings s USING(user_id) WHERE u.user_id=sqlc.arg(user_id);

-- name: SearchSocialUser :many
SELECT user_id FROM users WHERE account_status='active' AND
 ((sqlc.arg(user_id)::text<>'' AND user_id=sqlc.arg(user_id)) OR
  (sqlc.arg(username)::text<>'' AND username=sqlc.arg(username))) LIMIT 1;

-- name: LockSocialPair :many
SELECT user_id FROM users WHERE user_id IN(sqlc.arg(user1)::varchar,sqlc.arg(user2)::varchar)
ORDER BY user_id FOR UPDATE;

-- name: LatestSocialRequest :one
SELECT * FROM friend_request WHERE
 (from_user_id=sqlc.arg(user1) AND to_user_id=sqlc.arg(user2)) OR
 (from_user_id=sqlc.arg(user2) AND to_user_id=sqlc.arg(user1))
ORDER BY createtime DESC,request_id DESC LIMIT 1;

-- name: LockSocialRequest :one
SELECT * FROM friend_request WHERE request_id=$1 FOR UPDATE;

-- name: SetSocialRequestStatus :one
UPDATE friend_request SET status=sqlc.arg(status),handled_at=now()
WHERE request_id=sqlc.arg(request_id) AND status='pending' RETURNING *;

-- name: SocialRequestList :many
SELECT fr.request_id,fr.from_user_id,fr.to_user_id,COALESCE(fr.message,'')::text AS message,
 fr.status,fr.createtime,fr.handled_at,u.user_id,u.username,
 COALESCE(u.nickname,u.username)::text AS nickname,COALESCE(u.avatar_url,'')::text AS avatar,
 COALESCE(u.bio,'')::text AS signature,COALESCE(u.account_status::text,'active')::text AS account_status,
 COALESCE(u.online_status::text,'offline')::text AS online_status,
 COALESCE(s.show_online_status,true)::boolean AS show_online_status
FROM friend_request fr JOIN users u ON u.user_id=CASE WHEN fr.from_user_id=sqlc.arg(viewer_id) THEN fr.to_user_id ELSE fr.from_user_id END
LEFT JOIN user_settings s ON s.user_id=u.user_id
WHERE ((sqlc.arg(direction)::text='received' AND fr.to_user_id=sqlc.arg(viewer_id)) OR
 (sqlc.arg(direction)::text='sent' AND fr.from_user_id=sqlc.arg(viewer_id)))
AND (sqlc.arg(status_filter)::text='' OR fr.status=sqlc.arg(status_filter))
ORDER BY fr.createtime DESC,fr.request_id DESC LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountSocialRequestList :one
SELECT count(*) FROM friend_request fr WHERE
 ((sqlc.arg(direction)::text='received' AND fr.to_user_id=sqlc.arg(viewer_id)) OR
 (sqlc.arg(direction)::text='sent' AND fr.from_user_id=sqlc.arg(viewer_id)))
AND (sqlc.arg(status_filter)::text='' OR fr.status=sqlc.arg(status_filter));

-- name: SocialFriendList :many
SELECT u.user_id,u.username,COALESCE(u.nickname,u.username)::text AS nickname,
 COALESCE(u.avatar_url,'')::text AS avatar,COALESCE(u.bio,'')::text AS signature,
 COALESCE(u.account_status::text,'active')::text AS account_status,
 COALESCE(u.online_status::text,'offline')::text AS online_status,
 COALESCE(s.show_online_status,true)::boolean AS show_online_status,f.established_at
FROM friends f JOIN users u ON u.user_id=CASE WHEN f.user_id_1=sqlc.arg(viewer_id) THEN f.user_id_2 ELSE f.user_id_1 END
LEFT JOIN user_settings s ON s.user_id=u.user_id
WHERE (f.user_id_1=sqlc.arg(viewer_id) OR f.user_id_2=sqlc.arg(viewer_id))
AND (sqlc.arg(search)::text='' OR u.user_id ILIKE '%'||sqlc.arg(search)||'%' OR
 (u.account_status='active' AND (u.username ILIKE '%'||sqlc.arg(search)||'%' OR u.nickname ILIKE '%'||sqlc.arg(search)||'%')))
AND (sqlc.arg(status_filter)::text='all' OR (u.account_status='active' AND COALESCE(s.show_online_status,true) AND u.online_status::text=sqlc.arg(status_filter)))
ORDER BY f.established_at DESC,u.user_id ASC LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountSocialFriendList :one
SELECT count(*) FROM friends f JOIN users u ON u.user_id=CASE WHEN f.user_id_1=sqlc.arg(viewer_id) THEN f.user_id_2 ELSE f.user_id_1 END
LEFT JOIN user_settings s ON s.user_id=u.user_id
WHERE (f.user_id_1=sqlc.arg(viewer_id) OR f.user_id_2=sqlc.arg(viewer_id))
AND (sqlc.arg(search)::text='' OR u.user_id ILIKE '%'||sqlc.arg(search)||'%' OR
 (u.account_status='active' AND (u.username ILIKE '%'||sqlc.arg(search)||'%' OR u.nickname ILIKE '%'||sqlc.arg(search)||'%')))
AND (sqlc.arg(status_filter)::text='all' OR (u.account_status='active' AND COALESCE(s.show_online_status,true) AND u.online_status::text=sqlc.arg(status_filter)));

-- name: SocialFriendSince :one
SELECT established_at FROM friends WHERE user_id_1=LEAST(sqlc.arg(user1)::varchar,sqlc.arg(user2)::varchar)
AND user_id_2=GREATEST(sqlc.arg(user1)::varchar,sqlc.arg(user2)::varchar);

-- name: CountSocialRequestsToday :one
SELECT count(*) FROM friend_request WHERE from_user_id=$1 AND createtime>=(date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC');

-- name: InvalidateFriendInvitations :many
UPDATE chatroom_invitations SET status='invalidated',handled_at=now(),invalidation_reason='friendship_removed'
WHERE status='pending' AND ((inviter_id=sqlc.arg(user1) AND invitee_id=sqlc.arg(user2)) OR
(inviter_id=sqlc.arg(user2) AND invitee_id=sqlc.arg(user1))) RETURNING *;

-- name: ConversationKind :one
SELECT conversation_kind FROM chatrooms WHERE room_id=$1;

-- name: ExpireInvitations :exec
UPDATE chatroom_invitations SET status='expired',handled_at=now() WHERE status='pending' AND expires_at<=now();
