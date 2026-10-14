ALTER TABLE direct_pairs ADD COLUMN change_revision bigint NOT NULL DEFAULT 0 CHECK(change_revision>=0);
ALTER TABLE messages ADD COLUMN direct_revision bigint CHECK(direct_revision>0);
CREATE TABLE direct_message_changes (
 room_id varchar(9) NOT NULL REFERENCES direct_pairs(room_id),
 revision bigint NOT NULL CHECK(revision>0),
 message_id varchar(21) NOT NULL REFERENCES messages(message_id),
 action text NOT NULL CHECK(action IN ('new','edited','deleted')),
 created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(room_id,revision)
);
-- Preserve existing messages as a current-state recovery baseline.
UPDATE messages SET direct_revision=direct_seq WHERE direct_seq IS NOT NULL;
UPDATE direct_pairs p SET last_message_seq=GREATEST(p.last_message_seq,COALESCE((SELECT max(m.direct_seq) FROM messages m WHERE m.room_id=p.room_id),0)),
 change_revision=COALESCE((SELECT max(m.direct_revision) FROM messages m WHERE m.room_id=p.room_id),0);
INSERT INTO direct_message_changes(room_id,revision,message_id,action)
SELECT room_id,direct_revision,message_id,CASE WHEN deleted_at IS NULL THEN 'new' ELSE 'deleted' END FROM messages WHERE direct_revision IS NOT NULL;
CREATE INDEX idx_direct_changes_message ON direct_message_changes(message_id);
CREATE FUNCTION validate_direct_change() RETURNS trigger AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM messages WHERE message_id=NEW.message_id AND room_id=NEW.room_id AND direct_revision IS NOT NULL)
 THEN RAISE EXCEPTION 'Change must belong to its direct message'; END IF; RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER validate_direct_change BEFORE INSERT OR UPDATE ON direct_message_changes FOR EACH ROW EXECUTE FUNCTION validate_direct_change();
-- Transport code cannot accidentally insert an unversioned direct message.
CREATE FUNCTION require_direct_revision() RETURNS trigger AS $$ BEGIN
 IF (SELECT conversation_kind FROM chatrooms WHERE room_id=NEW.room_id)='direct' AND NEW.direct_revision IS NULL
 THEN RAISE EXCEPTION 'Direct messages require a revision'; END IF;
 IF (SELECT conversation_kind FROM chatrooms WHERE room_id=NEW.room_id)='group' AND NEW.direct_revision IS NOT NULL
 THEN RAISE EXCEPTION 'Direct revision on group message'; END IF; RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER require_direct_revision BEFORE INSERT OR UPDATE ON messages FOR EACH ROW EXECUTE FUNCTION require_direct_revision();
CREATE FUNCTION validate_direct_ledger() RETURNS trigger AS $$ BEGIN
 IF NEW.direct_revision IS NOT NULL AND NOT EXISTS(
 SELECT 1 FROM direct_message_changes ch JOIN direct_pairs p USING(room_id)
 WHERE ch.room_id=NEW.room_id AND ch.message_id=NEW.message_id AND ch.revision=NEW.direct_revision
 AND p.change_revision>=NEW.direct_revision AND p.last_message_seq>=NEW.direct_seq)
 THEN RAISE EXCEPTION 'Direct message must commit with its change ledger'; END IF; RETURN NULL;
END $$ LANGUAGE plpgsql;
CREATE CONSTRAINT TRIGGER validate_direct_ledger AFTER INSERT OR UPDATE ON messages DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION validate_direct_ledger();
