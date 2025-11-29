-- ----------------------------
-- 1. 创建枚举类型 (ENUMs)
-- ----------------------------

-- 用户在线状态
CREATE TYPE user_online_status AS ENUM (
    'online',
    'offline',
    'away',
    'do_not_disturb'
    );

-- 用户账号状态
CREATE TYPE user_account_status AS ENUM (
    'active',
    'suspended',
    'deleted',
    'pending_verification'
    );

-- 用户系统角色
CREATE TYPE user_system_role AS ENUM (
    'admin',
    'user'
    );

-- 聊天室类型
CREATE TYPE chatroom_type AS ENUM (
    'public',
    'private_password',
    'private_invite_only'
    );

-- 聊天室状态
CREATE TYPE chatroom_status AS ENUM (
    'active',
    'archived',
    'deleted'
    );

-- 消息类型
CREATE TYPE message_type AS ENUM (
    'text',
    'image',
    'file',
    'system_notification'
    );

-- 消息状态 (主要用于实现已读/未读等功能)
CREATE TYPE message_status AS ENUM (
    'sent',
    'delivered',
    'read',
    'failed',
    'deleted'
    );

-- 聊天室成员的角色
CREATE TYPE member_role AS ENUM (
    'owner',
    'admin',
    'member'
    );

-- 聊天室成员的禁言状态
CREATE TYPE member_mute_status AS ENUM (
    'not_muted',
    'muted'
    );

-- ----------------------------
-- 2. 创建表 (CREATE TABLE) - 英文命名
-- ----------------------------

-- 表: User (用户)

CREATE SEQUENCE user_idSeq
    START WITH 100000000
    INCREMENT BY 1
    MINVALUE 100000000;
CREATE OR REPLACE FUNCTION generateUserid()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('user_idSeq');

    NEW.user_id := 'U' || LPAD(next_id::text, 9, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE "users" (
                         "user_id" varchar(10) not null primary key, -- 用户编号
                         "username" VARCHAR(255) NOT NULL UNIQUE,             -- 用户名
                         "hashed_password" TEXT NOT NULL,                     -- 登录密码
                         "nickname" VARCHAR(255),                             -- 昵称
                         "phone_number" VARCHAR(50) UNIQUE,                   -- 手机号
                         "email" VARCHAR(255) UNIQUE,                         -- 电子邮箱
                         "avatar_url" TEXT DEFAULT 'https://example.com/default-avatar.jpg',                                   -- 头像
                         "bio" TEXT DEFAULT '这个人很懒，什么都没有留下~',                                          -- 个性签名
                         "online_status" user_online_status DEFAULT 'offline',-- 在线状态
                         "account_status" user_account_status DEFAULT 'pending_verification', -- 账号状态
                         "system_role" user_system_role DEFAULT 'user',       -- 系统角色
                         "registered_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, -- 注册时间
                         "last_login_at" TIMESTAMPTZ                           -- 最后登录时间
);
create trigger beforeInsertUser
    before insert on "users"
    for each row
execute function generateUserid();
-- 表: Chatroom (聊天室)
CREATE SEQUENCE chatroom_idSeq
    START WITH 100000000
    INCREMENT BY 1
    MINVALUE 100000000;
CREATE OR REPLACE FUNCTION generateChatroomID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('chatroom_idSeq');

    NEW.room_id := LPAD(next_id::text, 9, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE "chatrooms" (
                             "room_id" varchar(9)primary key , -- 聊天室编号
                             "room_name" VARCHAR(255) NOT NULL,                   -- 聊天室名称
                             "description" TEXT,                                  -- 聊天室描述
                             "icon_url" TEXT,                                     -- 聊天室图标
                             "room_type" chatroom_type NOT NULL DEFAULT 'public', -- 聊天室类型
                             "access_password" TEXT,                              -- 访问密码
                             "member_count" INTEGER NOT NULL DEFAULT 0,           -- 成员总数
                             "online_count" INTEGER NOT NULL DEFAULT 0,           -- 在线人数
                             "room_status" chatroom_status NOT NULL DEFAULT 'active', -- 聊天室状态
                             "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, -- 创建时间
                             "last_active_at" TIMESTAMPTZ                         -- 最后活跃时间
);

create trigger beforeInsertChatroom
    before insert on "chatrooms"
    for each row
execute function generateChatroomID();

CREATE OR REPLACE FUNCTION generateMemberRelID()
    RETURNS TRIGGER AS $$
BEGIN
    NEW.member_rel_id := 'M_' || NEW.user_id || '_' || NEW.room_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE "chatroom_members" (
                                    "member_rel_id" varchar(22)primary key, -- 成员关系编号
                                    "user_id" varchar(10) NOT NULL,                                   -- 用户编号
                                    "room_id" varchar(9) NOT NULL,                                   -- 聊天室编号
                                    "joined_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,  -- 加入时间
                                    "left_at" TIMESTAMPTZ,                                     -- 退出时间
                                    "last_read_at" TIMESTAMPTZ,                                -- 最后读取时间
                                    "member_role" member_role NOT NULL DEFAULT 'member',       -- 成员角色
                                    "mute_status" member_mute_status NOT NULL DEFAULT 'not_muted', -- 禁言状态
                                    "mute_expires_at" TIMESTAMPTZ,                             -- 禁言到期时间
                                    "is_active" BOOLEAN NOT NULL DEFAULT TRUE,                 -- 是否有效
                                    CONSTRAINT "chatroom_members_user_room_key" UNIQUE ("user_id", "room_id")
);
create trigger beforeInsertMemberRelID
    BEFORE INSERT ON "chatroom_members"
    FOR EACH ROW
EXECUTE FUNCTION generateMemberRelID();
-- 表: Message (消息)
CREATE SEQUENCE Message_idSeq
    START WITH 1
    INCREMENT BY 1
    MINVALUE 1;
CREATE OR REPLACE FUNCTION generateMessageID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('Message_idSeq');

    NEW.message_id :='M'||LPAD(next_id::text, 15, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE "messages" (
                            "message_id" varchar(16) PRIMARY KEY ,    -- 消息编号
                            "sent_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,    -- 发送时间
                            "content" TEXT NOT NULL,                                   -- 消息内容
                            "message_type" message_type NOT NULL DEFAULT 'text',       -- 消息类型
                            "quoted_message_id" varchar(9),                                  -- 引用消息编号
                            "sender_id" varchar(10),                                          -- 发送用户编号
                            "room_id" varchar(9) NOT NULL                                    -- 发送聊天室编号
);
create trigger beforeInsertMessage
    before insert on "messages"
    for each row
execute function generateMessageID();
-- 表: MuteRecord (禁言记录)
CREATE SEQUENCE MuteRecord_idSeq
    START WITH 100000000
    INCREMENT BY 1
    MINVALUE 100000000;
CREATE OR REPLACE FUNCTION generateMuteRecordID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('MuteRecord_idSeq');

    NEW.mute_record_id :=LPAD(next_id::text, 9, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE "mute_records" (
                                "mute_record_id" varchar(9) primary key , -- 禁言记录编号
                                "member_rel_id" varchar(22) NOT NULL,                                -- 成员关系编号
                                "start_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,    -- 禁言开始时间
                                "expires_at" TIMESTAMPTZ NOT NULL,                            -- 禁言结束时间
                                "reason" TEXT,                                                -- 禁言原因
                                "is_active" BOOLEAN NOT NULL DEFAULT TRUE,                    -- 是否生效
                                "admin_id" varchar(10)                                               -- 操作管理员编号
);
create trigger beforeInsertMuteRecord
    before insert on "mute_records"
    for each row
execute function generateMuteRecordID();
-- 表: GlobalMuteRecord (全局禁言记录)
CREATE SEQUENCE GlobalMuteRecord_idSeq
    START WITH 100000000
    INCREMENT BY 1
    MINVALUE 100000000;
CREATE OR REPLACE FUNCTION generateGlobalMuteRecordID()
    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('GlobalMuteRecord_idSeq');

    NEW.global_mute_id :=LPAD(next_id::text, 9, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE "global_mute_records" (
                                       "global_mute_id" varchar(9) PRIMARY KEY , -- 全局禁言记录编号
                                       "muted_user_id" varchar(10) not null,                                -- 禁言用户编号
                                       "start_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,    -- 禁言开始时间
                                       "expires_at" TIMESTAMPTZ NOT NULL,                            -- 禁言结束时间
                                       "reason" TEXT,                                                -- 禁言原因
                                       "is_active" BOOLEAN NOT NULL DEFAULT TRUE,                    -- 是否生效
                                       "admin_id" varchar(10)                                              -- 操作管理员编号
);
create trigger beforeInsertGlobalMuteRecord
    before insert on "global_mute_records"
    for each row
execute function generateGlobalMuteRecordID();
-- 表: AdminOperationLog (管理操作日志)

CREATE SEQUENCE Log_idSeq
    START WITH 100000000
    INCREMENT BY 1
    MINVALUE 100000000;
CREATE OR REPLACE FUNCTION generateLogID()

    RETURNS TRIGGER AS $$
DECLARE
    next_id BIGINT;
BEGIN
    next_id := nextval('Log_idSeq');

    NEW.log_id :=LPAD(next_id::text, 9, '0');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE "admin_logs" (
                              "log_id" varchar(9) primary key ,     -- 日志编号
                              "operator_user_id" varchar(10),                                  -- 操作用户编号
                              "operated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, -- 操作时间
                              "operation_type" VARCHAR(100) NOT NULL,                   -- 操作类型
                              "reason" TEXT,                                            -- 操作原因
                              "details" JSONB,                                          -- 操作详情
                              "is_global" BOOLEAN NOT NULL DEFAULT FALSE,               -- 是否全局操作
                              "related_room_id" varchar(9),                                   -- 关联聊天室编号
                              "related_user_id" varchar(10)                                    -- 关联用户编号
);
create trigger beforeInsertLog
    before insert on "admin_logs"
    for each row
execute function generateLogID();
-- ----------------------------
-- 3. 添加外键 (FOREIGN KEY) 约束 - 修正为英文
-- ----------------------------

ALTER TABLE "chatroom_members" ADD CONSTRAINT "fk_chatroom_members_user"
    FOREIGN KEY ("user_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "chatroom_members" ADD CONSTRAINT "fk_chatroom_members_room"
    FOREIGN KEY ("room_id") REFERENCES "chatrooms"("room_id") ON DELETE CASCADE;

ALTER TABLE "messages" ADD CONSTRAINT "fk_messages_quoted_message"
    FOREIGN KEY ("quoted_message_id") REFERENCES "messages"("message_id") ON DELETE SET NULL;

ALTER TABLE "messages" ADD CONSTRAINT "fk_messages_sender"
    FOREIGN KEY ("sender_id") REFERENCES "users"("user_id") ON DELETE SET NULL;

ALTER TABLE "messages" ADD CONSTRAINT "fk_messages_room"
    FOREIGN KEY ("room_id") REFERENCES "chatrooms"("room_id") ON DELETE CASCADE;

ALTER TABLE "mute_records" ADD CONSTRAINT "fk_mute_records_member"
    FOREIGN KEY ("member_rel_id") REFERENCES "chatroom_members"("member_rel_id") ON DELETE CASCADE;

ALTER TABLE "mute_records" ADD CONSTRAINT "fk_mute_records_admin"
    FOREIGN KEY ("admin_id") REFERENCES "chatroom_members"("member_rel_id") ON DELETE SET NULL;

ALTER TABLE "global_mute_records" ADD CONSTRAINT "fk_global_mute_records_user"
    FOREIGN KEY ("muted_user_id") REFERENCES "users"("user_id") ON DELETE CASCADE;

ALTER TABLE "global_mute_records" ADD CONSTRAINT "fk_global_mute_records_admin"
    FOREIGN KEY ("admin_id") REFERENCES "users"("user_id") ON DELETE SET NULL;

ALTER TABLE "admin_logs" ADD CONSTRAINT "fk_admin_logs_operator"
    FOREIGN KEY ("operator_user_id") REFERENCES "users"("user_id") ON DELETE SET NULL;

ALTER TABLE "admin_logs" ADD CONSTRAINT "fk_admin_logs_related_room"
    FOREIGN KEY ("related_room_id") REFERENCES "chatrooms"("room_id") ON DELETE SET NULL;

ALTER TABLE "admin_logs" ADD CONSTRAINT "fk_admin_logs_related_user"
    FOREIGN KEY ("related_user_id") REFERENCES "users"("user_id") ON DELETE SET NULL;


-- ----------------------------
-- 4. 创建索引 (INDEX) - 修正为英文
-- ----------------------------

CREATE INDEX "idx_users_email" ON "users" ("email");
CREATE INDEX "idx_users_phone_number" ON "users" ("phone_number");
CREATE INDEX "idx_users_account_status" ON "users" ("account_status");

CREATE INDEX "idx_chatrooms_room_type" ON "chatrooms" ("room_type");

CREATE INDEX "idx_chatroom_members_user_id" ON "chatroom_members" ("user_id");
CREATE INDEX "idx_chatroom_members_room_id" ON "chatroom_members" ("room_id");
CREATE INDEX "idx_chatroom_members_last_read_at" ON "chatroom_members" ("last_read_at");

CREATE INDEX "idx_messages_sent_at" ON "messages" ("sent_at" DESC);
CREATE INDEX "idx_messages_sender_id" ON "messages" ("sender_id");
CREATE INDEX "idx_messages_room_id" ON "messages" ("room_id");

CREATE INDEX "idx_mute_records_member_rel_id" ON "mute_records" ("member_rel_id");

CREATE INDEX "idx_global_mute_records_muted_user_id" ON "global_mute_records" ("muted_user_id");

CREATE INDEX "idx_admin_logs_operator_user_id" ON "admin_logs" ("operator_user_id");
CREATE INDEX "idx_admin_logs_operation_type" ON "admin_logs" ("operation_type");
CREATE INDEX "idx_admin_logs_operated_at" ON "admin_logs" ("operated_at" DESC);



