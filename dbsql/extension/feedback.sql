-- =============================================
-- 反馈建议相关SQL查询 (Feedback Queries)
-- 对应API: 反馈系统接口
-- 注意: 对应 migration/000002_first_newfunction.up.sql 中的表结构
-- =============================================

-- =============================================
-- 表结构说明 (已在migration中创建)
-- =============================================
-- feedbacks: 反馈表
--   - feedback_id (varchar 12, PK, 格式: FB00000000001)
--   - user_id (varchar 10, FK -> users, 可选)
--   - type (feedback_type ENUM: bug/feature/other)
--   - title (varchar 255)
--   - content (TEXT)
--   - contact_email (varchar 255, 可选)
--   - submitted_at (TIMESTAMPTZ)
--   - status (varchar 20, 默认 'new')

-- =============================================
-- 1. 反馈创建 (Feedback Creation)
-- =============================================

-- name: CreateFeedback :one
-- 创建反馈 POST /feedback
INSERT INTO feedbacks (
    user_id,
    type,
    title,
    content,
    contact_email
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING 
    feedback_id,
    user_id,
    type,
    title,
    content,
    contact_email,
    submitted_at,
    status;

-- name: CreateAnonymousFeedback :one
-- 创建匿名反馈（不需要登录）
INSERT INTO feedbacks (
    type,
    title,
    content,
    contact_email
) VALUES (
    $1, $2, $3, $4
) RETURNING 
    feedback_id,
    user_id,
    type,
    title,
    content,
    contact_email,
    submitted_at,
    status;

-- name: CreateBugReport :one
-- 创建Bug报告
INSERT INTO feedbacks (
    user_id,
    type,
    title,
    content,
    contact_email
) VALUES (
    $1, 'bug', $2, $3, $4
) RETURNING 
    feedback_id,
    user_id,
    type,
    title,
    content,
    contact_email,
    submitted_at,
    status;

-- name: CreateFeatureRequest :one
-- 创建功能建议
INSERT INTO feedbacks (
    user_id,
    type,
    title,
    content,
    contact_email
) VALUES (
    $1, 'feature', $2, $3, $4
) RETURNING 
    feedback_id,
    user_id,
    type,
    title,
    content,
    contact_email,
    submitted_at,
    status;

-- =============================================
-- 2. 反馈查询 (Feedback Queries)
-- =============================================

-- name: GetFeedbackByID :one
-- 获取反馈详情
SELECT 
    feedback_id,
    user_id,
    type,
    title,
    content,
    contact_email,
    submitted_at,
    status
FROM feedbacks 
WHERE feedback_id = $1;

-- name: GetFeedbacksByUser :many
-- 获取用户提交的反馈列表
SELECT 
    feedback_id,
    user_id,
    type,
    title,
    content,
    contact_email,
    submitted_at,
    status
FROM feedbacks 
WHERE user_id = $1
ORDER BY submitted_at DESC
LIMIT $2 OFFSET $3;

-- name: GetFeedbacksByType :many
-- 按类型获取反馈列表（管理员使用）
SELECT 
    f.feedback_id,
    f.user_id,
    f.type,
    f.title,
    f.content,
    f.contact_email,
    f.submitted_at,
    f.status,
    u.username,
    u.nickname
FROM feedbacks f
LEFT JOIN users u ON f.user_id = u.user_id
WHERE f.type = $1
ORDER BY f.submitted_at DESC
LIMIT $2 OFFSET $3;

-- name: GetFeedbacksByStatus :many
-- 按状态获取反馈列表（管理员使用）
SELECT 
    f.feedback_id,
    f.user_id,
    f.type,
    f.title,
    f.content,
    f.contact_email,
    f.submitted_at,
    f.status,
    u.username,
    u.nickname
FROM feedbacks f
LEFT JOIN users u ON f.user_id = u.user_id
WHERE f.status = $1
ORDER BY f.submitted_at DESC
LIMIT $2 OFFSET $3;

-- name: GetAllFeedbacks :many
-- 获取所有反馈（管理员使用）
SELECT 
    f.feedback_id,
    f.user_id,
    f.type,
    f.title,
    f.content,
    f.contact_email,
    f.submitted_at,
    f.status,
    u.username,
    u.nickname
FROM feedbacks f
LEFT JOIN users u ON f.user_id = u.user_id
ORDER BY f.submitted_at DESC
LIMIT $1 OFFSET $2;

-- name: GetNewFeedbacks :many
-- 获取新提交的反馈（status='new'）
SELECT 
    f.feedback_id,
    f.user_id,
    f.type,
    f.title,
    f.content,
    f.contact_email,
    f.submitted_at,
    f.status,
    u.username,
    u.nickname
FROM feedbacks f
LEFT JOIN users u ON f.user_id = u.user_id
WHERE f.status = 'new'
ORDER BY f.submitted_at ASC
LIMIT $1 OFFSET $2;

-- name: SearchFeedbacks :many
-- 搜索反馈（按标题和内容）
SELECT 
    f.feedback_id,
    f.user_id,
    f.type,
    f.title,
    f.content,
    f.contact_email,
    f.submitted_at,
    f.status,
    u.username,
    u.nickname
FROM feedbacks f
LEFT JOIN users u ON f.user_id = u.user_id
WHERE f.title ILIKE '%' || $1 || '%' 
    OR f.content ILIKE '%' || $1 || '%'
ORDER BY f.submitted_at DESC
LIMIT $2 OFFSET $3;

-- =============================================
-- 3. 反馈状态管理 (Feedback Status Management)
-- =============================================

-- name: UpdateFeedbackStatus :one
-- 更新反馈状态
UPDATE feedbacks 
SET status = $2
WHERE feedback_id = $1
RETURNING 
    feedback_id,
    user_id,
    type,
    title,
    content,
    contact_email,
    submitted_at,
    status;

-- name: MarkFeedbackAsReviewed :exec
-- 标记反馈为已审阅
UPDATE feedbacks 
SET status = 'reviewed'
WHERE feedback_id = $1 AND status = 'new';

-- name: MarkFeedbackAsInProgress :exec
-- 标记反馈为处理中
UPDATE feedbacks 
SET status = 'in_progress'
WHERE feedback_id = $1;

-- name: MarkFeedbackAsCompleted :exec
-- 标记反馈为已完成
UPDATE feedbacks 
SET status = 'completed'
WHERE feedback_id = $1;

-- name: MarkFeedbackAsClosed :exec
-- 标记反馈为已关闭
UPDATE feedbacks 
SET status = 'closed'
WHERE feedback_id = $1;

-- name: DeleteFeedback :exec
-- 删除反馈（慎用）
DELETE FROM feedbacks 
WHERE feedback_id = $1;

-- name: DeleteOldFeedbacks :exec
-- 删除超过指定天数的已完成反馈
DELETE FROM feedbacks 
WHERE status IN ('completed', 'closed') 
    AND submitted_at < NOW() - ($1 || ' days')::INTERVAL;

-- =============================================
-- 4. 反馈统计 (Feedback Statistics)
-- =============================================

-- name: CountFeedbacksByUser :one
-- 统计用户提交的反馈数量
SELECT COUNT(*) 
FROM feedbacks 
WHERE user_id = $1;

-- name: CountFeedbacksByType :one
-- 统计各类型反馈数量
SELECT COUNT(*) 
FROM feedbacks 
WHERE type = $1;

-- name: CountFeedbacksByStatus :one
-- 统计各状态反馈数量
SELECT COUNT(*) 
FROM feedbacks 
WHERE status = $1;

-- name: CountNewFeedbacks :one
-- 统计新反馈数量
SELECT COUNT(*) 
FROM feedbacks 
WHERE status = 'new';

-- name: GetFeedbackStats :one
-- 获取反馈统计（总数、各类型数量、各状态数量）
SELECT 
    COUNT(*) AS total_count,
    COUNT(*) FILTER (WHERE type = 'bug') AS bug_count,
    COUNT(*) FILTER (WHERE type = 'feature') AS feature_count,
    COUNT(*) FILTER (WHERE type = 'other') AS other_count,
    COUNT(*) FILTER (WHERE status = 'new') AS new_count,
    COUNT(*) FILTER (WHERE status = 'reviewed') AS reviewed_count,
    COUNT(*) FILTER (WHERE status = 'in_progress') AS in_progress_count,
    COUNT(*) FILTER (WHERE status = 'completed') AS completed_count,
    COUNT(*) FILTER (WHERE status = 'closed') AS closed_count
FROM feedbacks;

-- name: GetMostActiveFeedbackUsers :many
-- 获取提交反馈最多的用户
SELECT 
    f.user_id,
    u.username,
    u.nickname,
    COUNT(*) AS feedback_count
FROM feedbacks f
JOIN users u ON f.user_id = u.user_id
WHERE f.user_id IS NOT NULL
GROUP BY f.user_id, u.username, u.nickname
ORDER BY feedback_count DESC
LIMIT $1;

-- name: GetRecentFeedbackTrend :many
-- 获取最近反馈趋势（按日期分组）
SELECT 
    DATE(submitted_at) AS feedback_date,
    COUNT(*) AS count,
    COUNT(*) FILTER (WHERE type = 'bug') AS bug_count,
    COUNT(*) FILTER (WHERE type = 'feature') AS feature_count,
    COUNT(*) FILTER (WHERE type = 'other') AS other_count
FROM feedbacks 
WHERE submitted_at >= NOW() - INTERVAL '30 days'
GROUP BY DATE(submitted_at)
ORDER BY feedback_date DESC;
