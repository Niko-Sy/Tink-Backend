ALTER TABLE chatrooms ADD COLUMN conversation_kind text NOT NULL DEFAULT 'group' CHECK(conversation_kind IN ('group','direct'));
CREATE TABLE direct_pairs (
 room_id varchar(9) PRIMARY KEY REFERENCES chatrooms(room_id) ON DELETE RESTRICT,
 user_id_1 varchar(10) NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
 user_id_2 varchar(10) NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT now(),
 last_message_seq bigint NOT NULL DEFAULT 0 CHECK(last_message_seq>=0),
 CHECK(user_id_1<user_id_2), UNIQUE(user_id_1,user_id_2)
);
ALTER TABLE chatroom_members ADD COLUMN last_read_seq bigint NOT NULL DEFAULT 0 CHECK(last_read_seq>=0);
ALTER TABLE messages ADD COLUMN client_message_id uuid;
ALTER TABLE messages ADD COLUMN request_hash char(64);
ALTER TABLE messages ADD COLUMN direct_seq bigint CHECK(direct_seq>0);
ALTER TABLE messages ADD COLUMN edited_at timestamptz;
ALTER TABLE messages ADD COLUMN deleted_at timestamptz;
CREATE UNIQUE INDEX uq_message_client_key ON messages(room_id,sender_id,client_message_id) WHERE client_message_id IS NOT NULL;
CREATE UNIQUE INDEX uq_message_direct_seq ON messages(room_id,direct_seq) WHERE direct_seq IS NOT NULL;
CREATE INDEX idx_direct_history ON messages(room_id,direct_seq DESC) WHERE direct_seq IS NOT NULL;
CREATE FUNCTION immutable_conversation_kind() RETURNS trigger AS $$ BEGIN
 IF NEW.conversation_kind<>OLD.conversation_kind THEN RAISE EXCEPTION 'Conversation kind is immutable'; END IF;
 RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER immutable_conversation_kind BEFORE UPDATE ON chatrooms FOR EACH ROW EXECUTE FUNCTION immutable_conversation_kind();
CREATE FUNCTION validate_direct_membership() RETURNS trigger AS $$
DECLARE rid text; ids text[]; cr chatrooms%ROWTYPE; pair direct_pairs%ROWTYPE;
BEGIN
 IF TG_OP='INSERT' THEN ids:=ARRAY[NEW.room_id];
 ELSIF TG_OP='DELETE' THEN ids:=ARRAY[OLD.room_id];
 ELSE ids:=ARRAY[OLD.room_id,NEW.room_id]; END IF;
 FOREACH rid IN ARRAY ids LOOP
  SELECT * INTO cr FROM chatrooms WHERE room_id=rid;
  IF NOT FOUND THEN CONTINUE; END IF;
  SELECT * INTO pair FROM direct_pairs WHERE room_id=rid;
  IF cr.conversation_kind='group' THEN
   IF FOUND THEN RAISE EXCEPTION 'Pair must reference a direct conversation'; END IF;
  ELSE
   IF NOT FOUND OR cr.room_type<>'private_invite_only' OR cr.access_password IS NOT NULL OR cr.room_status<>'active' THEN
    RAISE EXCEPTION 'Invalid direct conversation'; END IF;
   IF (SELECT count(*) FROM chatroom_members WHERE room_id=rid)<>2 OR
    EXISTS(SELECT 1 FROM chatroom_members WHERE room_id=rid AND
      (NOT is_active OR member_role<>'member' OR user_id NOT IN(pair.user_id_1,pair.user_id_2))) OR cr.member_count<>2 THEN
    RAISE EXCEPTION 'Direct conversation must have exactly its two active members'; END IF;
  END IF;
 END LOOP;
 RETURN NULL;
END $$ LANGUAGE plpgsql;
CREATE CONSTRAINT TRIGGER validate_direct_room AFTER INSERT OR UPDATE ON chatrooms DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION validate_direct_membership();
CREATE CONSTRAINT TRIGGER validate_direct_pair AFTER INSERT OR UPDATE OR DELETE ON direct_pairs DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION validate_direct_membership();
CREATE CONSTRAINT TRIGGER validate_direct_member AFTER INSERT OR UPDATE OR DELETE ON chatroom_members DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION validate_direct_membership();
CREATE FUNCTION validate_message_conversation() RETURNS trigger AS $$ BEGIN
 IF (SELECT conversation_kind FROM chatrooms WHERE room_id=NEW.room_id)='direct' THEN
  IF NEW.direct_seq IS NULL OR NEW.client_message_id IS NULL OR NEW.request_hash IS NULL OR
    NEW.sender_id IS NULL OR NOT EXISTS(SELECT 1 FROM direct_pairs WHERE room_id=NEW.room_id AND NEW.sender_id IN(user_id_1,user_id_2)) THEN
   RAISE EXCEPTION 'Invalid direct message'; END IF;
 ELSIF NEW.direct_seq IS NOT NULL THEN RAISE EXCEPTION 'Direct sequence on group message'; END IF;
 RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER validate_message_conversation BEFORE INSERT OR UPDATE ON messages FOR EACH ROW EXECUTE FUNCTION validate_message_conversation();
