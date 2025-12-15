-- =============================================
-- 举报系统相关SQL查询 (Report Queries)
-- 对应API: 举报系统接口
-- 注意: 对应 migration/000002_first_newfunction.up.sql 中的表结构
-- =============================================

-- =============================================
-- 表结构说明 (已在migration中创建)
-- =============================================
-- reports: 举报表
--   - report_id (varchar 11, PK, 格式: RP0000000001)
--   - reporter_id (varchar 10, FK -> users)
--   - target_type (report_type ENUM: user/message)
--   - target_id (varchar 21)
--   - room_id (varchar 9, FK -> chatrooms, 可选)
--   - reason (report_reason ENUM: spam/harassment/inappropriate/other)
--   - description (TEXT)
--   - status (report_status ENUM: pending/resolved/rejected)
--   - created_at (TIMESTAMPTZ)
--   - resloved_at (TIMESTAMPTZ) ← 注意拼写错误 (resolved)
--   - resolved_by (varchar 10, FK -> users)

-- =============================================
-- 1. 举报创建 (Report Creation)
-- =============================================

-- name: CreateReport :one
-- 创建举报 POST /reports
INSERT INTO reports (
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING 
    report_id,
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description,
    status,
    created_at,
    resloved_at,
    resolved_by;

-- name: CreateUserReport :one
-- 创建用户举报
INSERT INTO reports (
    reporter_id,
    target_type,
    target_id,
    reason,
    description
) VALUES (
    $1, 'user', $2, $3, $4
) RETURNING 
    report_id,
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description,
    status,
    created_at,
    resloved_at,
    resolved_by;

-- name: CreateMessageReport :one
-- 创建消息举报（需要提供聊天室ID）
INSERT INTO reports (
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description
) VALUES (
    $1, 'message', $2, $3, $4, $5
) RETURNING 
    report_id,
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description,
    status,
    created_at,
    resloved_at,
    resolved_by;

-- =============================================
-- 2. 举报查询 (Report Queries)
-- =============================================

-- name: GetReportByID :one
-- 获取举报详情
SELECT 
    report_id,
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description,
    status,
    created_at,
    resloved_at,
    resolved_by
FROM reports 
WHERE report_id = $1;

-- name: GetReportsByReporter :many
-- 获取用户提交的举报列表
SELECT 
    report_id,
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description,
    status,
    created_at,
    resloved_at,
    resolved_by
FROM reports 
WHERE reporter_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetReportsByStatus :many
-- 按状态获取举报列表（管理员使用）
SELECT 
    r.report_id,
    r.reporter_id,
    r.target_type,
    r.target_id,
    r.room_id,
    r.reason,
    r.description,
    r.status,
    r.created_at,
    r.resloved_at,
    r.resolved_by,
    u.username AS reporter_username,
    u.nickname AS reporter_nickname
FROM reports r
JOIN users u ON r.reporter_id = u.user_id
WHERE r.status = $1
ORDER BY r.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPendingReports :many
-- 获取待处理的举报（管理员使用）
SELECT 
    r.report_id,
    r.reporter_id,
    r.target_type,
    r.target_id,
    r.room_id,
    r.reason,
    r.description,
    r.status,
    r.created_at,
    r.resloved_at,
    r.resolved_by,
    u.username AS reporter_username,
    u.nickname AS reporter_nickname
FROM reports r
JOIN users u ON r.reporter_id = u.user_id
WHERE r.status = 'pending'
ORDER BY r.created_at ASC
LIMIT $1 OFFSET $2;

-- name: GetReportsByTargetType :many
-- 按目标类型获取举报列表（管理员使用）
SELECT 
    r.report_id,
    r.reporter_id,
    r.target_type,
    r.target_id,
    r.room_id,
    r.reason,
    r.description,
    r.status,
    r.created_at,
    r.resloved_at,
    r.resolved_by,
    u.username AS reporter_username,
    u.nickname AS reporter_nickname
FROM reports r
JOIN users u ON r.reporter_id = u.user_id
WHERE r.target_type = $1
ORDER BY r.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetReportsAgainstTarget :many
-- 获取针对特定目标的所有举报
SELECT 
    report_id,
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description,
    status,
    created_at,
    resloved_at,
    resolved_by
FROM reports 
WHERE target_id = $1 AND target_type = $2
ORDER BY created_at DESC;

-- name: CheckDuplicateReport :one
-- 检查是否存在重复举报
SELECT EXISTS(
    SELECT 1 FROM reports 
    WHERE reporter_id = $1 
        AND target_type = $2 
        AND target_id = $3 
        AND status = 'pending'
) AS has_pending_report;

-- =============================================
-- 3. 举报处理 (Report Handling)
-- =============================================

-- name: ResolveReport :one
-- 处理举报（标记为已处理）
UPDATE reports 
SET 
    status = 'resolved',
    resloved_at = NOW(),
    resolved_by = $2
WHERE report_id = $1 AND status = 'pending'
RETURNING 
    report_id,
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description,
    status,
    created_at,
    resloved_at,
    resolved_by;

-- name: RejectReport :one
-- 拒绝举报
UPDATE reports 
SET 
    status = 'rejected',
    resloved_at = NOW(),
    resolved_by = $2
WHERE report_id = $1 AND status = 'pending'
RETURNING 
    report_id,
    reporter_id,
    target_type,
    target_id,
    room_id,
    reason,
    description,
    status,
    created_at,
    resloved_at,
    resolved_by;

-- name: DeleteReport :exec
-- 删除举报（慎用）
DELETE FROM reports 
WHERE report_id = $1;

-- =============================================
-- 4. 举报统计 (Report Statistics)
-- =============================================

-- name: CountReportsByReporter :one
-- 统计用户提交的举报数量
SELECT COUNT(*) 
FROM reports 
WHERE reporter_id = $1;

-- name: CountReportsByStatus :one
-- 统计各状态举报数量
SELECT COUNT(*) 
FROM reports 
WHERE status = $1;

-- name: CountPendingReports :one
-- 统计待处理举报数量
SELECT COUNT(*) 
FROM reports 
WHERE status = 'pending';

-- name: CountReportsAgainstTarget :one
-- 统计针对特定目标的举报数量
SELECT COUNT(*) 
FROM reports 
WHERE target_id = $1 AND target_type = $2;

-- name: GetReportStats :one
-- 获取举报统计（总数、待处理、已处理、已拒绝）
SELECT 
    COUNT(*) AS total_count,
    COUNT(*) FILTER (WHERE status = 'pending') AS pending_count,
    COUNT(*) FILTER (WHERE status = 'resolved') AS resolved_count,
    COUNT(*) FILTER (WHERE status = 'rejected') AS rejected_count,
    COUNT(*) FILTER (WHERE target_type = 'user') AS user_report_count,
    COUNT(*) FILTER (WHERE target_type = 'message') AS message_report_count
FROM reports;

-- name: GetMostReportedUsers :many
-- 获取被举报次数最多的用户
SELECT 
    target_id AS user_id,
    COUNT(*) AS report_count
FROM reports 
WHERE target_type = 'user'
GROUP BY target_id
ORDER BY report_count DESC
LIMIT $1;

-- name: GetMostReportedMessages :many
-- 获取被举报次数最多的消息
SELECT 
    target_id AS message_id,
    room_id,
    COUNT(*) AS report_count
FROM reports 
WHERE target_type = 'message'
GROUP BY target_id, room_id
ORDER BY report_count DESC
LIMIT $1;
