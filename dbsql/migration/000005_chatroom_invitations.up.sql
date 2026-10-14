CREATE TABLE chatroom_invitations (
 invitation_id uuid PRIMARY KEY,
 room_id varchar(9) NOT NULL REFERENCES chatrooms(room_id),
 inviter_id varchar(10) NOT NULL REFERENCES users(user_id),
 invitee_id varchar(10) NOT NULL REFERENCES users(user_id),
 message text NOT NULL DEFAULT '' CHECK(char_length(message)<=200),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','accepted','rejected','cancelled','expired','invalidated')),
 created_at timestamptz NOT NULL DEFAULT now(),expires_at timestamptz NOT NULL DEFAULT now()+interval '7 days',
 handled_at timestamptz,invalidation_reason text,
 CHECK(inviter_id<>invitee_id),CHECK(expires_at>created_at),CHECK((status='pending')=(handled_at IS NULL))
);
CREATE UNIQUE INDEX uq_invitation_pending ON chatroom_invitations(room_id,invitee_id) WHERE status='pending';
CREATE INDEX idx_invitation_received ON chatroom_invitations(invitee_id,status,created_at DESC,invitation_id DESC);
CREATE INDEX idx_invitation_sent ON chatroom_invitations(inviter_id,status,created_at DESC,invitation_id DESC);
CREATE INDEX idx_invitation_expiry ON chatroom_invitations(expires_at) WHERE status='pending';
CREATE FUNCTION validate_invitation_group() RETURNS trigger AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM chatrooms WHERE room_id=NEW.room_id AND conversation_kind='group') THEN
  RAISE EXCEPTION 'Invitations require a group conversation'; END IF; RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER validate_invitation_group BEFORE INSERT OR UPDATE ON chatroom_invitations FOR EACH ROW EXECUTE FUNCTION validate_invitation_group();
