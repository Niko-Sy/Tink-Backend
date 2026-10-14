DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM friend_request WHERE status='cancelled') OR EXISTS(
 SELECT 1 FROM friend_request GROUP BY from_user_id,to_user_id HAVING count(*)>1)
 THEN RAISE EXCEPTION 'Unsafe rollback: retain social request history and use a compatible application'; END IF;
END $$;
DROP INDEX idx_friends_user2;
DROP INDEX idx_friend_request_sent;
DROP INDEX idx_friend_request_received;
DROP INDEX uq_friend_request_pending_pair;
ALTER TABLE friend_request DROP CONSTRAINT friend_request_handled_check;
ALTER TABLE friend_request DROP CONSTRAINT friend_request_not_self;
ALTER TABLE friend_request DROP CONSTRAINT friend_request_status_check;
ALTER TABLE friend_request ALTER COLUMN status DROP DEFAULT;
ALTER TABLE friend_request ALTER COLUMN status TYPE friend_request_status USING status::friend_request_status;
ALTER TABLE friend_request ALTER COLUMN status SET DEFAULT 'pending';
ALTER TABLE friend_request ADD UNIQUE(from_user_id,to_user_id);
-- Audit evidence deliberately survives rollback.
