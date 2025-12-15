-- =============================================
-- 文件上传管理相关SQL查询 (Uploaded Files Queries)
-- 对应API: 文件管理接口
-- 注意: 对应 migration/000002_first_newfunction.up.sql 中的表结构
-- =============================================

-- =============================================
-- 表结构说明 (已在migration中创建)
-- =============================================
-- uploaded_files: 上传文件表
--   - file_id (varchar 11, PK, 格式: F0000000001)
--   - uploader_id (varchar 10, FK -> users)
--   - file_name (varchar 255)
--   - file_path (varchar 512)
--   - file_size (bigint)
--   - mime_type (varchar 255)
--   - upload_context (uploaded_file_type ENUM: avatar/chat_image/file)
--   - related_room_id (varchar 9, FK -> chatrooms, 可选)
--   - uploaded_at (TIMESTAMPTZ)
--   - is_deleted (boolean)

-- =============================================
-- 1. 文件记录创建 (File Record Creation)
-- =============================================

-- name: CreateFileRecord :one
-- 创建文件上传记录
INSERT INTO uploaded_files (
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted;

-- name: CreateAvatarRecord :one
-- 创建头像文件记录
INSERT INTO uploaded_files (
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context
) VALUES (
    $1, $2, $3, $4, $5, 'avatar'
) RETURNING 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted;

-- name: CreateChatImageRecord :one
-- 创建聊天图片记录
INSERT INTO uploaded_files (
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id
) VALUES (
    $1, $2, $3, $4, $5, 'chat_image', $6
) RETURNING 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted;

-- =============================================
-- 2. 文件记录查询 (File Record Queries)
-- =============================================

-- name: GetFileByID :one
-- 获取文件记录详情
SELECT 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted
FROM uploaded_files 
WHERE file_id = $1;

-- name: GetFilesByUploader :many
-- 获取用户上传的文件列表
SELECT 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted
FROM uploaded_files 
WHERE uploader_id = $1 AND is_deleted = false
ORDER BY uploaded_at DESC
LIMIT $2 OFFSET $3;

-- name: GetFilesByRoom :many
-- 获取聊天室的文件列表
SELECT 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted
FROM uploaded_files 
WHERE related_room_id = $1 AND is_deleted = false
ORDER BY uploaded_at DESC
LIMIT $2 OFFSET $3;

-- name: GetFilesByContext :many
-- 按上传类型获取文件列表
SELECT 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted
FROM uploaded_files 
WHERE upload_context = $1 AND is_deleted = false
ORDER BY uploaded_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUserAvatars :many
-- 获取用户的头像历史记录
SELECT 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted
FROM uploaded_files 
WHERE uploader_id = $1 AND upload_context = 'avatar' AND is_deleted = false
ORDER BY uploaded_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUserLatestAvatar :one
-- 获取用户最新的头像
SELECT 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted
FROM uploaded_files 
WHERE uploader_id = $1 AND upload_context = 'avatar' AND is_deleted = false
ORDER BY uploaded_at DESC
LIMIT 1;

-- name: GetFileByPath :one
-- 根据文件路径获取文件记录
SELECT 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted
FROM uploaded_files 
WHERE file_path = $1;

-- =============================================
-- 3. 文件记录管理 (File Record Management)
-- =============================================

-- name: MarkFileAsDeleted :exec
-- 标记文件为已删除（软删除）
UPDATE uploaded_files 
SET is_deleted = true
WHERE file_id = $1 AND uploader_id = $2;

-- name: DeleteFileRecord :exec
-- 删除文件记录（硬删除，慎用）
DELETE FROM uploaded_files 
WHERE file_id = $1 AND uploader_id = $2;

-- name: DeleteFilesByUploader :exec
-- 删除用户的所有文件记录（用户注销时）
DELETE FROM uploaded_files 
WHERE uploader_id = $1;

-- name: DeleteOldDeletedFiles :exec
-- 删除超过指定天数的已标记删除的文件记录
DELETE FROM uploaded_files 
WHERE is_deleted = true 
    AND uploaded_at < NOW() - ($1 || ' days')::INTERVAL;

-- name: RestoreDeletedFile :exec
-- 恢复已删除的文件
UPDATE uploaded_files 
SET is_deleted = false
WHERE file_id = $1 AND uploader_id = $2;

-- =============================================
-- 4. 文件统计 (File Statistics)
-- =============================================

-- name: CountFilesByUploader :one
-- 统计用户上传的文件数量
SELECT COUNT(*) 
FROM uploaded_files 
WHERE uploader_id = $1 AND is_deleted = false;

-- name: CountFilesByRoom :one
-- 统计聊天室的文件数量
SELECT COUNT(*) 
FROM uploaded_files 
WHERE related_room_id = $1 AND is_deleted = false;

-- name: CountFilesByContext :one
-- 统计各类型文件数量
SELECT COUNT(*) 
FROM uploaded_files 
WHERE upload_context = $1 AND is_deleted = false;

-- name: GetUserStorageSize :one
-- 获取用户已使用的存储空间大小
SELECT COALESCE(SUM(file_size), 0) AS total_size
FROM uploaded_files 
WHERE uploader_id = $1 AND is_deleted = false;

-- name: GetRoomStorageSize :one
-- 获取聊天室文件占用的存储空间大小
SELECT COALESCE(SUM(file_size), 0) AS total_size
FROM uploaded_files 
WHERE related_room_id = $1 AND is_deleted = false;

-- name: GetTotalStorageSize :one
-- 获取系统总存储空间使用
SELECT COALESCE(SUM(file_size), 0) AS total_size
FROM uploaded_files 
WHERE is_deleted = false;

-- name: GetFileStats :one
-- 获取文件统计（总数、各类型数量、总大小）
SELECT 
    COUNT(*) AS total_count,
    COUNT(*) FILTER (WHERE upload_context = 'avatar') AS avatar_count,
    COUNT(*) FILTER (WHERE upload_context = 'chat_image') AS chat_image_count,
    COUNT(*) FILTER (WHERE upload_context = 'file') AS file_count,
    COALESCE(SUM(file_size), 0) AS total_size,
    COALESCE(SUM(file_size) FILTER (WHERE upload_context = 'avatar'), 0) AS avatar_size,
    COALESCE(SUM(file_size) FILTER (WHERE upload_context = 'chat_image'), 0) AS chat_image_size,
    COALESCE(SUM(file_size) FILTER (WHERE upload_context = 'file'), 0) AS file_size_sum
FROM uploaded_files 
WHERE is_deleted = false;

-- name: GetLargestFiles :many
-- 获取最大的文件列表
SELECT 
    file_id,
    uploader_id,
    file_name,
    file_path,
    file_size,
    mime_type,
    upload_context,
    related_room_id,
    uploaded_at,
    is_deleted
FROM uploaded_files 
WHERE is_deleted = false
ORDER BY file_size DESC
LIMIT $1;

-- name: GetMostActiveUploaders :many
-- 获取上传文件最多的用户
SELECT 
    uploader_id,
    COUNT(*) AS file_count,
    COALESCE(SUM(file_size), 0) AS total_size
FROM uploaded_files 
WHERE is_deleted = false
GROUP BY uploader_id
ORDER BY file_count DESC
LIMIT $1;

-- name: GetRecentUploadTrend :many
-- 获取最近上传趋势（按日期分组）
SELECT 
    DATE(uploaded_at) AS upload_date,
    COUNT(*) AS count,
    COALESCE(SUM(file_size), 0) AS total_size,
    COUNT(*) FILTER (WHERE upload_context = 'avatar') AS avatar_count,
    COUNT(*) FILTER (WHERE upload_context = 'chat_image') AS chat_image_count,
    COUNT(*) FILTER (WHERE upload_context = 'file') AS file_count
FROM uploaded_files 
WHERE uploaded_at >= NOW() - INTERVAL '30 days' AND is_deleted = false
GROUP BY DATE(uploaded_at)
ORDER BY upload_date DESC;
