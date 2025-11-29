drop function generateUserid() cascade;
drop function generateChatroomID() cascade;
drop function generateMessageID() cascade;
drop function generateMuteRecordID() cascade;
drop function generateGlobalMuteRecordID() cascade;
drop function generateLogID() cascade;
drop function generateMemberRelID() cascade;
drop sequence user_idSeq;
drop sequence chatroom_idSeq;
drop sequence Message_idSeq;
drop sequence MuteRecord_idSeq;
drop sequence GlobalMuteRecord_idSeq;
drop sequence Log_idSeq;
-- ----------------------------
-- 1. 删除所有外键引用的表 (Deepest Dependencies)
-- ----------------------------

-- 表: mute_records (禁言记录) - 引用 chatroom_members
DROP TABLE IF EXISTS "mute_records" CASCADE;

-- 表: global_mute_records (全局禁言记录) - 引用 users
DROP TABLE IF EXISTS "global_mute_records" CASCADE;

-- 表: admin_logs (管理操作日志) - 引用 users, chatrooms
DROP TABLE IF EXISTS "admin_logs" CASCADE;

-- 表: messages (消息) - 引用 messages, users, chatrooms
DROP TABLE IF EXISTS "messages" CASCADE;

-- 表: chatroom_members (聊天室成员) - 引用 users, chatrooms
DROP TABLE IF EXISTS "chatroom_members" CASCADE;


-- ----------------------------
-- 2. 删除父表 (Parent Tables)
-- ----------------------------

-- 表: chatrooms (聊天室)
DROP TABLE IF EXISTS "chatrooms" CASCADE;

-- 表: users (用户)
DROP TABLE IF EXISTS "users" CASCADE;


-- ----------------------------
-- 3. 删除自定义类型 (ENUMs)
-- ----------------------------
-- 必须在所有使用这些类型的表被删除之后执行。
DROP TYPE IF EXISTS member_mute_status;      -- 聊天室成员的禁言状态
DROP TYPE IF EXISTS member_role;             -- 聊天室成员的角色
DROP TYPE IF EXISTS message_status;          -- 消息状态 (已在消息表创建时删除)
DROP TYPE IF EXISTS message_type;            -- 消息类型
DROP TYPE IF EXISTS chatroom_status;         -- 聊天室状态
DROP TYPE IF EXISTS chatroom_type;           -- 聊天室类型
DROP TYPE IF EXISTS user_system_role;        -- 用户系统角色
DROP TYPE IF EXISTS user_account_status;     -- 用户账号状态
DROP TYPE IF EXISTS user_online_status;      -- 用户在线状态


