--删除函数和序列
DROP FUNCTION generateFriendRequestID() CASCADE ;
DROP FUNCTION generateFriendID() CASCADE ;
DROP FUNCTION generateNotificationID() CASCADE ;
DROP FUNCTION generateUploadedFileID() CASCADE ;
DROP FUNCTION generateReportID() CASCADE;
DROP FUNCTION generateFeedbackID() CASCADE ;

DROP SEQUENCE "friend_request_id_seq" CASCADE ;
DROP SEQUENCE "friend_id_seq" CASCADE ;
DROP SEQUENCE "notification_id_seq" CASCADE ;
DROP SEQUENCE "uploaded_files_id_seq" CASCADE ;
DROP SEQUENCE "report_id_seq" CASCADE;
DROP SEQUENCE "feedback_id_seq" CASCADE ;

--删除表
DROP TABLE IF EXISTS "friend_request" CASCADE ;
DROP TABLE IF EXISTS "friends" CASCADE ;
DROP TABLE IF EXISTS "notifications" CASCADE ;
DROP TABLE IF EXISTS "user_settings" CASCADE ;
DROP TABLE IF EXISTS "uploaded_files" CASCADE ;
DROP TABLE IF EXISTS "reports" CASCADE ;
DROP TABLE IF EXISTS "feedbacks" CASCADE ;
--删除自定义类型
DROP TYPE IF EXISTS friend_request_status;
DROP TYPE IF EXISTS notification_type;
DROP TYPE IF EXISTS uploaded_file_type;
DROP TYPE IF EXISTS report_type;
DROP TYPE IF EXISTS report_reason;
DROP TYPE IF EXISTS report_status;
DROP TYPE IF EXISTS feedback_type;