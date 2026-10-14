-- name: GetChatroomInvitation :one
SELECT * FROM chatroom_invitations WHERE invitation_id=$1;
-- name: LockInvitation :one
SELECT * FROM chatroom_invitations WHERE invitation_id=$1 FOR UPDATE;
-- name: LatestInvitation :one
SELECT * FROM chatroom_invitations WHERE room_id=$1 AND invitee_id=$2 ORDER BY created_at DESC,invitation_id DESC LIMIT 1;
-- name: PendingInvitation :one
SELECT * FROM chatroom_invitations WHERE room_id=$1 AND invitee_id=$2 AND status='pending';
-- name: CreateInvitation :one
INSERT INTO chatroom_invitations(invitation_id,room_id,inviter_id,invitee_id,message) VALUES($1,$2,$3,$4,$5) RETURNING *;
-- name: TransitionInvitation :one
UPDATE chatroom_invitations SET status=$2,handled_at=now(),invalidation_reason=$3 WHERE invitation_id=$1 AND status='pending' RETURNING *;
-- name: EffectiveInvitationStatus :one
SELECT invitation_effective_status(i,clock_timestamp())::text FROM chatroom_invitations i WHERE invitation_id=$1;
-- name: InvitationClock :one
SELECT clock_timestamp()::timestamptz;
-- name: InvitationList :many
SELECT i.*,invitation_effective_status(i)::text AS effective_status FROM chatroom_invitations i
WHERE ((sqlc.arg(scope)::text='received' AND invitee_id=sqlc.arg(viewer)) OR
(sqlc.arg(scope)::text='sent' AND inviter_id=sqlc.arg(viewer)) OR
(sqlc.arg(scope)::text='room' AND room_id=sqlc.arg(room) AND (sqlc.arg(manage)::boolean OR inviter_id=sqlc.arg(viewer))))
AND (sqlc.arg(status_filter)::text='' OR invitation_effective_status(i)=sqlc.arg(status_filter))
ORDER BY created_at DESC,invitation_id DESC LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
-- name: CountInvitationList :one
SELECT count(*) FROM chatroom_invitations i
WHERE ((sqlc.arg(scope)::text='received' AND invitee_id=sqlc.arg(viewer)) OR
(sqlc.arg(scope)::text='sent' AND inviter_id=sqlc.arg(viewer)) OR
(sqlc.arg(scope)::text='room' AND room_id=sqlc.arg(room) AND (sqlc.arg(manage)::boolean OR inviter_id=sqlc.arg(viewer))))
AND (sqlc.arg(status_filter)::text='' OR invitation_effective_status(i)=sqlc.arg(status_filter));
-- name: CountPendingInvitations :one
SELECT count(*) FROM chatroom_invitations i WHERE invitee_id=$1 AND invitation_effective_status(i)='pending';
-- name: CountInvitationsToday :one
SELECT count(*) FROM chatroom_invitations WHERE inviter_id=$1 AND created_at>=(date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC');
-- name: StaleInvitations :many
SELECT * FROM chatroom_invitations i WHERE status='pending' AND invitation_effective_status(i)<>'pending'
ORDER BY created_at,invitation_id LIMIT $1;
-- name: InvalidateMemberInvitations :many
UPDATE chatroom_invitations SET status='invalidated',handled_at=now(),invalidation_reason='membership_changed'
WHERE status='pending' AND room_id=$1 AND (inviter_id=$2 OR invitee_id=$2) RETURNING *;
-- name: InvalidateRoomInvitations :many
UPDATE chatroom_invitations SET status='invalidated',handled_at=now(),invalidation_reason='room_deleted'
WHERE status='pending' AND room_id=$1 RETURNING *;
-- name: InvalidateUnauthorizedInvitations :many
UPDATE chatroom_invitations i SET status='invalidated',handled_at=now(),invalidation_reason='invite_permission_changed'
WHERE status='pending' AND room_id=$1 AND invitation_effective_status(i)='invalidated' RETURNING i.*;
-- name: InvitableRooms :many
SELECT r.* FROM chatrooms r JOIN chatroom_members m USING(room_id)
WHERE r.conversation_kind='group' AND r.room_status='active' AND m.user_id=sqlc.arg(viewer) AND m.is_active
AND (r.room_type='public' OR m.member_role IN ('owner','admin'))
AND (sqlc.arg(search)::text='' OR r.room_name ILIKE '%'||sqlc.arg(search)||'%' OR r.room_id=sqlc.arg(search))
ORDER BY r.created_at DESC,r.room_id LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
-- name: CountInvitableRooms :one
SELECT count(*) FROM chatrooms r JOIN chatroom_members m USING(room_id)
WHERE r.conversation_kind='group' AND r.room_status='active' AND m.user_id=sqlc.arg(viewer) AND m.is_active
AND (r.room_type='public' OR m.member_role IN ('owner','admin'))
AND (sqlc.arg(search)::text='' OR r.room_name ILIKE '%'||sqlc.arg(search)||'%' OR r.room_id=sqlc.arg(search));
-- name: LockGroupRoom :one
SELECT * FROM chatrooms WHERE room_id=$1 AND conversation_kind='group' AND room_status='active' FOR UPDATE;
-- name: GroupActiveMembers :many
SELECT user_id FROM chatroom_members WHERE room_id=$1 AND is_active ORDER BY user_id;
-- name: DeactivateGroupMembers :exec
UPDATE chatroom_members SET is_active=false,left_at=now() WHERE room_id=$1 AND is_active;
-- name: ResetDeletedGroupCounts :exec
UPDATE chatrooms SET member_count=0,online_count=0 WHERE room_id=$1 AND conversation_kind='group';
-- name: CountInvitationsMinute :one
SELECT count(*) FROM chatroom_invitations WHERE inviter_id=$1 AND created_at>now()-interval '1 minute';
-- name: LockFreeGroupRoom :one
SELECT * FROM chatrooms WHERE room_id=$1 AND conversation_kind='group' AND room_status='active';
-- name: GroupManagers :many
SELECT user_id FROM chatroom_members WHERE room_id=$1 AND is_active AND member_role IN ('owner','admin') ORDER BY user_id;
