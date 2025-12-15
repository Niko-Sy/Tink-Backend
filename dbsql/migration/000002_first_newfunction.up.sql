--好友请求状态
CREATE TYPE friend_request_status AS ENUM (
    'pending',
    'accepted',
    'rejected'
    );

--通知的类型
CREATE TYPE notification_type AS ENUM (
    'friend',
    'chatroom',
    'system'
    );

CREATE TYPE uploaded_file_type AS ENUM (
    'avatar',
    'chat_image',
    'file'
    );
CREATE TYPE report_type AS ENUM(
    'user',
    'message'
    );
CREATE TYPE report_reason AS ENUM (
    'spam',
    'harassment',
    'inappropriate',
    'other'
    );
CREATE TYPE report_status AS ENUM (
    'pending' ,
    'resolved' ,
    'rejected'
    );
CREATE TYPE feedback_type AS ENUM (
    'bug' ,
    'feature' ,
    'other'
    );
-- 表: friendRequest (好友请求)
CREATE SEQUENCE friend_request_id_seq
    START WITH 1
    INCREMENT BY 1
    MINVALUE 1;
CREATE OR REPLACE FUNCTION generateFriendRequestID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('friend_request_id_seq');

    NEW.request_id :='FR'||LPAD(next_id::text, 10, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


CREATE TABLE "friend_request"(
                                 "request_id" varchar(12) PRIMARY KEY ,
                                 "from_user_id" varchar(10) NOT NULL,
                                 "to_user_id" varchar(10) NOT NULL,
                                 "message" TEXT,
                                 "status" friend_request_status NOT NULL default 'pending',
                                 "createtime" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                 "handled_at" TIMESTAMPTZ,
                                 UNIQUE ("from_user_id", "to_user_id")
);
CREATE TRIGGER beforeInsertFriendRequest
    before insert on "friend_request"
    for each row
execute function generateFriendRequestID();




--表: friendrelations (好友关系表)
CREATE SEQUENCE friend_id_seq
    START WITH 1
    INCREMENT BY 1
    MINVALUE 1;
CREATE OR REPLACE FUNCTION generateFriendID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('friend_id_seq');

    NEW.relation_id :='R'||LPAD(next_id::text, 10, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TABLE "friends"(
                          "relation_id" varchar(11) PRIMARY KEY,
                          "user_id_1" varchar(10) NOT NULL,
                          "user_id_2" varchar(10) NOT NULL,
                          "established_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                          CHECK("user_id_1" < "user_id_2"),
                          UNIQUE ("user_id_1", "user_id_2")
);
CREATE TRIGGER beforeInsertFriend
    before insert on "friends"
    for each row
execute function generateFriendID();




--表:notification (消息通知表)
CREATE SEQUENCE notification_id_seq
    START WITH 1
    INCREMENT BY 1
    MINVALUE 1;
CREATE OR REPLACE FUNCTION generateNotificationID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('notification_id_seq');

    NEW.notification_id :='N'||LPAD(next_id::text, 10, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TABLE "notifications"(
                                "notification_id" varchar(11) PRIMARY KEY,
                                "user_id" varchar(10) NOT NULL,
                                "type" notification_type NOT NULL,
                                "title" varchar(255) NOT NULL,
                                "content" TEXT NOT NULL,
                                "data" JSONB,
                                "is_read" boolean NOT NULL DEFAULT FALSE,
                                "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER beforeInsertNotification
    before insert on "notifications"
    for each row
execute function generateNotificationID();

--表:user_settings(用户设置）
CREATE TABLE "user_settings"(
                                "user_id" varchar(10) PRIMARY KEY,
                                "enable_friend_request" boolean NOT NULL DEFAULT TRUE,
                                "enable_chatroom_messages" boolean NOT NULL DEFAULT TRUE,
                                "enable_system_notice" boolean NOT NULL DEFAULT TRUE,
                                "enable_sound" boolean NOT NULL DEFAULT TRUE,
                                "enable_desktop_notification" boolean NOT NULL DEFAULT TRUE,
                                "allow_search_by_phone" boolean NOT NULL DEFAULT TRUE,
                                "allow_search_by_email" boolean NOT NULL DEFAULT TRUE,
                                "show_online_status" boolean NOT NULL DEFAULT TRUE,
                                "update_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

--表: uploaded_files (上传的文件)
CREATE SEQUENCE "uploaded_files_id_seq"
    START WITH 1
    INCREMENT BY 1
    MINVALUE 1;
CREATE OR REPLACE FUNCTION generateUploadedFileID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('"uploaded_files_id_seq"');

    NEW.file_id :='F'||LPAD(next_id::text, 10, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TABLE "uploaded_files"(
    "file_id" varchar(11) PRIMARY KEY,
    "uploader_id" varchar(10) NOT NULL,
    "file_name" varchar(255) NOT NULL,
    "file_path" varchar(512) NOT NULL,
    "file_size" bigint NOT NULL,
    "mime_type" varchar(255) NOT NULL,
    "upload_context" uploaded_file_type NOT NULL,
    "related_room_id" varchar(9),
    "uploaded_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "is_deleted" boolean NOT NULL DEFAULT FALSE
);
CREATE TRIGGER beforeInsertUploadedFile
    before insert on "uploaded_files"
    for each row
    execute function generateUploadedFileID();


--表: reports (举报表)
CREATE SEQUENCE "report_id_seq"
    START WITH 1
    INCREMENT BY 1
    MINVALUE 1;
CREATE OR REPLACE FUNCTION generateReportID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('"report_id_seq"');

    NEW.report_id :='RP'||LPAD(next_id::text, 10, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE "reports"(
    "report_id" varchar(11) PRIMARY KEY,
    "reporter_id" varchar(10) NOT NULL,
    "target_type" report_type NOT NULL,
    "target_id" varchar(21) NOT NULL,
    "room_id" varchar(9),
    "reason" report_reason NOT NULL,
    "description" TEXT,
    "status" report_status NOT NULL DEFAULT 'pending',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "resloved_at" TIMESTAMPTZ,
    "resolved_by" varchar(10)
);
CREATE TRIGGER beforeInsertReport
    before insert on "reports"
    for each row
execute function generateReportID();

--表: feedbacks (反馈表)
CREATE SEQUENCE "feedback_id_seq"
    START WITH 1
    INCREMENT BY 1
    MINVALUE 1;
CREATE OR REPLACE FUNCTION generateFeedbackID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('"feedback_id_seq"');

    NEW.feedback_id :='FB'||LPAD(next_id::text, 11, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE "feedbacks"(
    "feedback_id" VARCHAR(12) PRIMARY KEY,
    "user_id" VARCHAR(10) ,
    "type" feedback_type NOT NULL,
    "title" VARCHAR(255) NOT NULL,
    "content" TEXT NOT NULL,
    "contact_email" VARCHAR(255),
    "submitted_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "status" VARCHAR(20) DEFAULT 'new'
);
CREATE TRIGGER beforeInsertFeedback
    before insert on "feedbacks"
    for each row
execute function generateFeedbackID();
--添加外键约束
ALTER TABLe "friend_request" ADD CONSTRAINT "fk_friend_request_from_user"
    FOREIGN KEY ("from_user_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "friend_request" ADD CONSTRAINT "fk_friend_request_to_user"
    FOREIGN KEY ("to_user_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "friends" ADD CONSTRAINT "fk_friends_user_id_1"
    FOREIGN KEY ("user_id_1") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "friends" ADD CONSTRAINT "fk_friends_user_id_2"
    FOREIGN KEY ("user_id_2") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "notifications" ADD CONSTRAINT "fk_notifications_user"
    FOREIGN KEY ("user_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "user_settings" ADD CONSTRAINT "fk_user_settings_user"
    FOREIGN KEY ("user_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "uploaded_files" ADD CONSTRAINT "fk_uploaded_files_uploader"
    FOREIGN KEY ("uploader_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "uploaded_files" ADD CONSTRAINT "fk_uploaded_files_room"
    FOREIGN KEY ("related_room_id") REFERENCES "chatrooms"("room_id") ON DELETE CASCADE;

ALTER TABLE "reports" ADD CONSTRAINT "fk_reports_reporter"
    FOREIGN KEY ("reporter_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "reports" ADD CONSTRAINT "fk_reports_target"
    FOREIGN KEY ("target_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "reports" ADD CONSTRAINT "fk_reports_room"
    FOREIGN KEY ("room_id") REFERENCES "chatrooms"("room_id") ON DELETE CASCADE;

ALTER TABLE "reports" ADD CONSTRAINT "fk_reports_resolved_by"
    FOREIGN KEY ("resolved_by") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "feedbacks" ADD CONSTRAINT "fk_feedbacks_user"
    FOREIGN KEY ("user_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

--创建索引
CREATE INDEX "idx_notifications_user_id" ON "notifications" ("user_id");
CREATE INDEX "idx_notifications_is_read" ON "notifications" ("is_read");

--修改现有表
--ALTER TABLE "messages"
