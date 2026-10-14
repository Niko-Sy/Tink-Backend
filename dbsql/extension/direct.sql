-- Direct conversation queries. Binding and write policies live in service/direct.
-- name: GetDirectPair :one
SELECT * FROM direct_pairs WHERE user_id_1=LEAST(sqlc.arg(user1)::varchar,sqlc.arg(user2)::varchar)
AND user_id_2=GREATEST(sqlc.arg(user1)::varchar,sqlc.arg(user2)::varchar);

-- name: DirectPairByRoom :one
SELECT * FROM direct_pairs WHERE room_id=$1;

-- name: LockDirectRoom :one
SELECT p.* FROM direct_pairs p JOIN chatrooms c USING(room_id) WHERE p.room_id=$1 AND c.conversation_kind='direct' AND c.room_status='active' FOR UPDATE OF p,c;

-- name: CreateDirectRoom :one
INSERT INTO chatrooms(room_name,room_type,conversation_kind,member_count,last_active_at)
VALUES('私信','private_invite_only','direct',2,now()) RETURNING *;

-- name: CreateDirectPair :one
INSERT INTO direct_pairs(room_id,user_id_1,user_id_2)
VALUES(sqlc.arg(room_id),LEAST(sqlc.arg(user1)::varchar,sqlc.arg(user2)::varchar),GREATEST(sqlc.arg(user1)::varchar,sqlc.arg(user2)::varchar)) RETURNING *;

-- name: DirectConversationList :many
SELECT p.room_id,COALESCE(c.last_active_at,c.created_at)::timestamptz AS activity_at
FROM direct_pairs p JOIN chatrooms c USING(room_id)
WHERE sqlc.arg(viewer_id) IN(p.user_id_1,p.user_id_2) AND
 (sqlc.narg(cursor_time)::timestamptz IS NULL OR (COALESCE(c.last_active_at,c.created_at),p.room_id)<(sqlc.narg(cursor_time)::timestamptz,sqlc.arg(cursor_room)::varchar))
ORDER BY activity_at DESC,p.room_id DESC LIMIT sqlc.arg(page_limit);

-- name: AdvanceDirectCounters :one
UPDATE direct_pairs SET last_message_seq=last_message_seq+1,change_revision=change_revision+1 WHERE room_id=$1 RETURNING *;

-- name: AdvanceDirectRevision :one
UPDATE direct_pairs SET change_revision=change_revision+1 WHERE room_id=$1 RETURNING *;

-- name: TouchDirectRoom :exec
UPDATE chatrooms SET last_active_at=clock_timestamp() WHERE room_id=$1 AND conversation_kind='direct';

-- name: DirectActivity :one
SELECT created_at,COALESCE(last_active_at,created_at)::timestamptz AS activity_at FROM chatrooms WHERE room_id=$1 AND conversation_kind='direct';

-- name: InsertDirectMessage :one
INSERT INTO messages(room_id,sender_id,content,message_type,quoted_message_id,client_message_id,request_hash,direct_seq,direct_revision,sent_at)
VALUES(sqlc.arg(room_id),sqlc.arg(sender_id)::varchar,sqlc.arg(content),'text',sqlc.narg(quoted_message_id)::varchar,sqlc.arg(client_message_id)::uuid,sqlc.arg(request_hash)::text,sqlc.arg(direct_seq)::bigint,sqlc.arg(direct_revision)::bigint,clock_timestamp()) RETURNING *;

-- name: DirectMessageByKey :one
SELECT * FROM messages WHERE room_id=sqlc.arg(room_id) AND sender_id=sqlc.arg(sender_id)::varchar AND client_message_id=sqlc.arg(client_message_id)::uuid;

-- name: DirectMessageByID :one
SELECT * FROM messages WHERE room_id=sqlc.arg(room_id) AND message_id=sqlc.arg(message_id);

-- name: DirectHistoryBefore :many
SELECT * FROM messages WHERE room_id=sqlc.arg(room_id) AND direct_seq IS NOT NULL AND
 (sqlc.arg(before_seq)::bigint=0 OR direct_seq<sqlc.arg(before_seq)) ORDER BY direct_seq DESC LIMIT sqlc.arg(page_limit);

-- name: DirectHistoryAfter :many
SELECT * FROM messages WHERE room_id=sqlc.arg(room_id) AND direct_seq>sqlc.arg(after_seq)::bigint ORDER BY direct_seq ASC LIMIT sqlc.arg(page_limit);

-- name: LatestDirectMessage :one
SELECT * FROM messages WHERE room_id=$1 AND direct_seq IS NOT NULL ORDER BY direct_seq DESC LIMIT 1;

-- name: EditDirectMessage :one
UPDATE messages SET content=sqlc.arg(content),edited_at=clock_timestamp(),direct_revision=sqlc.arg(revision)::bigint
WHERE room_id=sqlc.arg(room_id) AND message_id=sqlc.arg(message_id) AND deleted_at IS NULL RETURNING *;

-- name: WithdrawDirectMessage :one
UPDATE messages SET content='',deleted_at=clock_timestamp(),direct_revision=sqlc.arg(revision)::bigint
WHERE room_id=sqlc.arg(room_id) AND message_id=sqlc.arg(message_id) AND deleted_at IS NULL RETURNING *;

-- name: RecordDirectChange :exec
INSERT INTO direct_message_changes(room_id,revision,message_id,action) VALUES($1,$2,$3,$4);

-- name: DirectChanges :many
SELECT * FROM direct_message_changes WHERE room_id=sqlc.arg(room_id) AND revision>sqlc.arg(after_revision)::bigint
AND revision<=sqlc.arg(up_to_revision)::bigint ORDER BY revision ASC LIMIT sqlc.arg(page_limit);

-- name: DirectReadPosition :one
SELECT last_read_seq FROM chatroom_members WHERE room_id=$1 AND user_id=$2;

-- name: AdvanceDirectRead :one
UPDATE chatroom_members SET last_read_seq=GREATEST(last_read_seq,sqlc.arg(target_seq)::bigint),last_read_at=clock_timestamp()
WHERE room_id=sqlc.arg(room_id) AND user_id=sqlc.arg(user_id) RETURNING last_read_seq;

-- name: DirectUnread :one
SELECT count(*) FROM messages m JOIN chatroom_members cm ON cm.room_id=m.room_id AND cm.user_id=sqlc.arg(viewer_id)
WHERE m.room_id=sqlc.arg(room_id) AND m.direct_seq>cm.last_read_seq AND m.sender_id<>sqlc.arg(viewer_id) AND m.deleted_at IS NULL;

-- name: TotalDirectUnread :one
SELECT count(*) FROM messages m JOIN direct_pairs p ON p.room_id=m.room_id JOIN chatroom_members cm ON cm.room_id=m.room_id AND cm.user_id=sqlc.arg(viewer_id)
WHERE sqlc.arg(viewer_id) IN(p.user_id_1,p.user_id_2) AND m.direct_seq>cm.last_read_seq AND m.sender_id<>sqlc.arg(viewer_id) AND m.deleted_at IS NULL;
