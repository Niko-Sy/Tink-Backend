-- =============================================
-- 管理操作日志相关SQL查询 (Admin Log Queries)
-- 对应API: 系统管理接口
-- =============================================

-- =============================================
-- 1. 日志创建 (Log Creation)
-- =============================================

-- name: CreateAdminLog :one
-- 创建管理操作日志
INSERT INTO admin_logs (
    operator_user_id,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING 
    log_id,
    operator_user_id,
    operated_at,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id;

-- name: CreateMuteLog :one
-- 创建禁言操作日志
INSERT INTO admin_logs (
    operator_user_id,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id
) VALUES (
    $1, 'mute', $2, $3, $4, $5, $6
) RETURNING 
    log_id,
    operator_user_id,
    operated_at,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id;

-- name: CreateUnmuteLog :one
-- 创建解除禁言操作日志
INSERT INTO admin_logs (
    operator_user_id,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id
) VALUES (
    $1, 'unmute', $2, $3, $4, $5, $6
) RETURNING 
    log_id,
    operator_user_id,
    operated_at,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id;

-- name: CreateKickLog :one
-- 创建踢人操作日志
INSERT INTO admin_logs (
    operator_user_id,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id
) VALUES (
    $1, 'kick', $2, $3, false, $4, $5
) RETURNING 
    log_id,
    operator_user_id,
    operated_at,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id;

-- name: CreateRoleChangeLog :one
-- 创建角色变更操作日志
INSERT INTO admin_logs (
    operator_user_id,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id
) VALUES (
    $1, 'role_change', $2, $3, false, $4, $5
) RETURNING 
    log_id,
    operator_user_id,
    operated_at,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id;

-- name: CreateBanLog :one
-- 创建封禁账号操作日志
INSERT INTO admin_logs (
    operator_user_id,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id
) VALUES (
    $1, 'ban', $2, $3, true, NULL, $4
) RETURNING 
    log_id,
    operator_user_id,
    operated_at,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id;

-- name: CreateDeleteMessageLog :one
-- 创建删除消息操作日志
INSERT INTO admin_logs (
    operator_user_id,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id
) VALUES (
    $1, 'delete_message', $2, $3, false, $4, $5
) RETURNING 
    log_id,
    operator_user_id,
    operated_at,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id;

-- =============================================
-- 2. 日志查询 (Log Queries)
-- =============================================

-- name: GetAdminLogByID :one
-- 获取单条日志
SELECT 
    log_id,
    operator_user_id,
    operated_at,
    operation_type,
    reason,
    details,
    is_global,
    related_room_id,
    related_user_id
FROM admin_logs 
WHERE log_id = $1;

-- name: GetAdminLogs :many
-- 获取所有管理日志（分页）
SELECT 
    al.log_id,
    al.operator_user_id,
    al.operated_at,
    al.operation_type,
    al.reason,
    al.details,
    al.is_global,
    al.related_room_id,
    al.related_user_id,
    ou.username AS operator_username,
    ou.nickname AS operator_nickname,
    ru.username AS related_username,
    ru.nickname AS related_nickname
FROM admin_logs al
LEFT JOIN users ou ON al.operator_user_id = ou.user_id
LEFT JOIN users ru ON al.related_user_id = ru.user_id
ORDER BY al.operated_at DESC
LIMIT $1 OFFSET $2;

-- name: GetAdminLogsByOperator :many
-- 获取指定操作员的日志
SELECT 
    al.log_id,
    al.operator_user_id,
    al.operated_at,
    al.operation_type,
    al.reason,
    al.details,
    al.is_global,
    al.related_room_id,
    al.related_user_id,
    ru.username AS related_username,
    ru.nickname AS related_nickname
FROM admin_logs al
LEFT JOIN users ru ON al.related_user_id = ru.user_id
WHERE al.operator_user_id = $1
ORDER BY al.operated_at DESC
LIMIT $2 OFFSET $3;

-- name: GetAdminLogsByRoom :many
-- 获取聊天室的管理日志
SELECT 
    al.log_id,
    al.operator_user_id,
    al.operated_at,
    al.operation_type,
    al.reason,
    al.details,
    al.is_global,
    al.related_room_id,
    al.related_user_id,
    ou.username AS operator_username,
    ou.nickname AS operator_nickname,
    ru.username AS related_username,
    ru.nickname AS related_nickname
FROM admin_logs al
LEFT JOIN users ou ON al.operator_user_id = ou.user_id
LEFT JOIN users ru ON al.related_user_id = ru.user_id
WHERE al.related_room_id = $1
ORDER BY al.operated_at DESC
LIMIT $2 OFFSET $3;

-- name: GetAdminLogsByUser :many
-- 获取涉及指定用户的日志
SELECT 
    al.log_id,
    al.operator_user_id,
    al.operated_at,
    al.operation_type,
    al.reason,
    al.details,
    al.is_global,
    al.related_room_id,
    al.related_user_id,
    ou.username AS operator_username,
    ou.nickname AS operator_nickname
FROM admin_logs al
LEFT JOIN users ou ON al.operator_user_id = ou.user_id
WHERE al.related_user_id = $1
ORDER BY al.operated_at DESC
LIMIT $2 OFFSET $3;

-- name: GetAdminLogsByType :many
-- 按操作类型获取日志
SELECT 
    al.log_id,
    al.operator_user_id,
    al.operated_at,
    al.operation_type,
    al.reason,
    al.details,
    al.is_global,
    al.related_room_id,
    al.related_user_id,
    ou.username AS operator_username,
    ou.nickname AS operator_nickname,
    ru.username AS related_username,
    ru.nickname AS related_nickname
FROM admin_logs al
LEFT JOIN users ou ON al.operator_user_id = ou.user_id
LEFT JOIN users ru ON al.related_user_id = ru.user_id
WHERE al.operation_type = $1
ORDER BY al.operated_at DESC
LIMIT $2 OFFSET $3;

-- name: GetGlobalAdminLogs :many
-- 获取全局管理日志
SELECT 
    al.log_id,
    al.operator_user_id,
    al.operated_at,
    al.operation_type,
    al.reason,
    al.details,
    al.is_global,
    al.related_room_id,
    al.related_user_id,
    ou.username AS operator_username,
    ou.nickname AS operator_nickname,
    ru.username AS related_username,
    ru.nickname AS related_nickname
FROM admin_logs al
LEFT JOIN users ou ON al.operator_user_id = ou.user_id
LEFT JOIN users ru ON al.related_user_id = ru.user_id
WHERE al.is_global = true
ORDER BY al.operated_at DESC
LIMIT $1 OFFSET $2;

-- name: GetAdminLogsByTimeRange :many
-- 按时间范围获取日志
SELECT 
    al.log_id,
    al.operator_user_id,
    al.operated_at,
    al.operation_type,
    al.reason,
    al.details,
    al.is_global,
    al.related_room_id,
    al.related_user_id,
    ou.username AS operator_username,
    ou.nickname AS operator_nickname,
    ru.username AS related_username,
    ru.nickname AS related_nickname
FROM admin_logs al
LEFT JOIN users ou ON al.operator_user_id = ou.user_id
LEFT JOIN users ru ON al.related_user_id = ru.user_id
WHERE al.operated_at >= $1 AND al.operated_at <= $2
ORDER BY al.operated_at DESC
LIMIT $3 OFFSET $4;

-- =============================================
-- 3. 日志统计 (Log Statistics)
-- =============================================

-- name: CountAdminLogs :one
-- 统计日志总数
SELECT COUNT(*) FROM admin_logs;

-- name: CountAdminLogsByOperator :one
-- 统计操作员的日志数
SELECT COUNT(*) FROM admin_logs WHERE operator_user_id = $1;

-- name: CountAdminLogsByRoom :one
-- 统计聊天室的日志数
SELECT COUNT(*) FROM admin_logs WHERE related_room_id = $1;

-- name: CountAdminLogsByType :one
-- 按类型统计日志数
SELECT COUNT(*) FROM admin_logs WHERE operation_type = $1;

-- name: GetAdminLogStats :many
-- 获取各类型操作的统计
SELECT 
    operation_type,
    COUNT(*) AS count
FROM admin_logs
GROUP BY operation_type
ORDER BY count DESC;

-- name: GetOperatorStats :many
-- 获取各操作员的操作统计
SELECT 
    al.operator_user_id,
    u.username,
    u.nickname,
    COUNT(*) AS operation_count
FROM admin_logs al
JOIN users u ON al.operator_user_id = u.user_id
GROUP BY al.operator_user_id, u.username, u.nickname
ORDER BY operation_count DESC
LIMIT $1;
