DO $$ BEGIN IF EXISTS(SELECT 1 FROM chatroom_invitations) THEN RAISE EXCEPTION 'Unsafe rollback: invitations exist'; END IF; END $$;
DROP TRIGGER validate_invitation_group ON chatroom_invitations;
DROP FUNCTION validate_invitation_group();
DROP TABLE chatroom_invitations;
