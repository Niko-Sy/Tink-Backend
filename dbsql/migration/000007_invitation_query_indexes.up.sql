CREATE INDEX idx_invitation_room ON chatroom_invitations(room_id,status,created_at DESC,invitation_id DESC);
CREATE INDEX idx_invitation_pair_history ON chatroom_invitations(room_id,invitee_id,created_at DESC,invitation_id DESC);
-- A single effective-state predicate keeps counts, pages and candidates consistent
-- even when the cleanup worker has not run yet. Terminal rows are immutable.
CREATE FUNCTION invitation_effective_status(i chatroom_invitations, at_time timestamptz DEFAULT now()) RETURNS text LANGUAGE sql STABLE AS $$
 SELECT CASE WHEN i.status <> 'pending' THEN i.status
 WHEN i.expires_at<=at_time THEN 'expired'
 WHEN NOT EXISTS (SELECT 1 FROM users a,users b,chatrooms r,chatroom_members m
 WHERE a.user_id=i.inviter_id AND b.user_id=i.invitee_id AND a.account_status='active' AND b.account_status='active'
 AND r.room_id=i.room_id AND r.conversation_kind='group' AND r.room_status='active'
 AND m.room_id=r.room_id AND m.user_id=a.user_id AND m.is_active
 AND (r.room_type='public' OR m.member_role IN ('owner','admin'))
 AND EXISTS(SELECT 1 FROM friends f WHERE f.user_id_1=LEAST(a.user_id,b.user_id) AND f.user_id_2=GREATEST(a.user_id,b.user_id)))
 THEN 'invalidated' ELSE 'pending' END;
$$;
