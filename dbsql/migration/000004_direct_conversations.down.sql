DO $$ BEGIN IF EXISTS(SELECT 1 FROM chatrooms WHERE conversation_kind='direct') THEN
 RAISE EXCEPTION 'Unsafe rollback: direct conversations exist'; END IF; END $$;
DROP TRIGGER validate_message_conversation ON messages;
DROP FUNCTION validate_message_conversation();
DROP TRIGGER validate_direct_member ON chatroom_members;
DROP TRIGGER validate_direct_pair ON direct_pairs;
DROP TRIGGER validate_direct_room ON chatrooms;
DROP FUNCTION validate_direct_membership();
DROP TRIGGER immutable_conversation_kind ON chatrooms;
DROP FUNCTION immutable_conversation_kind();
DROP TABLE direct_pairs;
DROP INDEX idx_direct_history;
DROP INDEX uq_message_direct_seq;
DROP INDEX uq_message_client_key;
ALTER TABLE messages DROP COLUMN deleted_at,DROP COLUMN edited_at,DROP COLUMN direct_seq,DROP COLUMN request_hash,DROP COLUMN client_message_id;
ALTER TABLE chatroom_members DROP COLUMN last_read_seq;
ALTER TABLE chatrooms DROP COLUMN conversation_kind;
