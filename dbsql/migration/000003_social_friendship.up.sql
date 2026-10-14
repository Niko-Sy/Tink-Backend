-- Retain a durable record of every legacy row repaired by this migration.
CREATE TABLE IF NOT EXISTS friend_request_migration_audit (
  request_id varchar(12) NOT NULL, reason text NOT NULL,
  original_row jsonb NOT NULL, recorded_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO friend_request_migration_audit
SELECT request_id, 'self_request', to_jsonb(fr), now()
FROM friend_request fr WHERE from_user_id = to_user_id;
DELETE FROM friend_request WHERE from_user_id = to_user_id;
ALTER TABLE friend_request ALTER COLUMN status DROP DEFAULT;
ALTER TABLE friend_request ALTER COLUMN status TYPE text USING status::text;
ALTER TABLE friend_request ALTER COLUMN status SET DEFAULT 'pending';
ALTER TABLE friend_request DROP CONSTRAINT friend_request_from_user_id_to_user_id_key;
WITH ranked AS (
 SELECT request_id, row_number() OVER (
 PARTITION BY LEAST(from_user_id,to_user_id),GREATEST(from_user_id,to_user_id)
 ORDER BY createtime,request_id) AS n FROM friend_request WHERE status='pending'
)
INSERT INTO friend_request_migration_audit
SELECT fr.request_id,'duplicate_pending',to_jsonb(fr),now()
FROM friend_request fr JOIN ranked r USING(request_id) WHERE r.n>1;
UPDATE friend_request SET status='cancelled',handled_at=now()
WHERE request_id IN (SELECT request_id FROM friend_request_migration_audit WHERE reason='duplicate_pending');
INSERT INTO friend_request_migration_audit
SELECT request_id,'inconsistent_handled_at',to_jsonb(fr),now() FROM friend_request fr
WHERE (status='pending' AND handled_at IS NOT NULL) OR (status<>'pending' AND handled_at IS NULL);
UPDATE friend_request SET handled_at=CASE WHEN status='pending' THEN NULL ELSE COALESCE(handled_at,createtime) END;
ALTER TABLE friend_request ADD CONSTRAINT friend_request_status_check CHECK(status IN ('pending','accepted','rejected','cancelled'));
ALTER TABLE friend_request ADD CONSTRAINT friend_request_not_self CHECK(from_user_id<>to_user_id);
ALTER TABLE friend_request ADD CONSTRAINT friend_request_handled_check CHECK((status='pending')=(handled_at IS NULL));
CREATE UNIQUE INDEX uq_friend_request_pending_pair ON friend_request(LEAST(from_user_id,to_user_id),GREATEST(from_user_id,to_user_id)) WHERE status='pending';
CREATE INDEX idx_friend_request_received ON friend_request(to_user_id,status,createtime DESC,request_id DESC);
CREATE INDEX idx_friend_request_sent ON friend_request(from_user_id,status,createtime DESC,request_id DESC);
CREATE INDEX idx_friends_user2 ON friends(user_id_2);
