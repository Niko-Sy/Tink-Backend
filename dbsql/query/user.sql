-- =============================================
-- 用户相关SQL查询 (User Queries)
-- 对应API: 认证相关接口 + 用户管理接口
-- =============================================

-- =============================================
-- 1. 认证相关 (Authentication)
-- =============================================

-- name: CreateUser :one
-- 用户注册 POST /auth/register
INSERT INTO users (
    username,
    hashed_password,
    nickname,
    phone_number,
    email,
    avatar_url,
    bio
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING 
    user_id,
    username,
    hashed_password,
    nickname,
    phone_number,
    email,
    avatar_url,
    bio,
    online_status,
    account_status,
    system_role,
    registered_at,
    last_login_at;

-- name: GetUserByID :one
-- 根据ID获取用户信息 GET /users/:userId
SELECT 
    user_id,
    username,
    hashed_password,
    nickname,
    phone_number,
    email,
    avatar_url,
    bio,
    online_status,
    account_status,
    system_role,
    registered_at,
    last_login_at
FROM users 
WHERE user_id = $1;

-- name: GetUserByUsername :one
-- 用户登录时通过用户名查找 POST /auth/login
SELECT 
    user_id,
    username,
    hashed_password,
    nickname,
    phone_number,
    email,
    avatar_url,
    bio,
    online_status,
    account_status,
    system_role,
    registered_at,
    last_login_at
FROM users 
WHERE username = $1;

-- name: GetUserByEmail :one
-- 用户登录时通过邮箱查找 POST /auth/login
SELECT 
    user_id,
    username,
    hashed_password,
    nickname,
    phone_number,
    email,
    avatar_url,
    bio,
    online_status,
    account_status,
    system_role,
    registered_at,
    last_login_at
FROM users 
WHERE email = $1;

-- name: CheckUsernameExists :one
-- 检查用户名是否已存在
SELECT EXISTS(SELECT 1 FROM users WHERE username = $1) AS exists;

-- name: CheckEmailExists :one
-- 检查邮箱是否已存在
SELECT EXISTS(SELECT 1 FROM users WHERE email = $1) AS exists;

-- name: CheckPhoneExists :one
-- 检查手机号是否已存在
SELECT EXISTS(SELECT 1 FROM users WHERE phone_number = $1) AS exists;

-- =============================================
-- 2. 用户信息管理 (User Profile Management)
-- =============================================

-- name: UpdateUser :one
-- 更新用户资料 PUT /users/me
UPDATE users 
SET 
    nickname = COALESCE($2, nickname),
    phone_number = COALESCE($3, phone_number),
    email = COALESCE($4, email),
    avatar_url = COALESCE($5, avatar_url),
    bio = COALESCE($6, bio)
WHERE user_id = $1
RETURNING 
    user_id,
    username,
    hashed_password,
    nickname,
    phone_number,
    email,
    avatar_url,
    bio,
    online_status,
    account_status,
    system_role,
    registered_at,
    last_login_at;

-- name: UpdateUserPassword :exec
-- 修改密码 POST /auth/change-password
UPDATE users 
SET hashed_password = $2
WHERE user_id = $1;

-- name: UpdateUserAvatar :exec
-- 更新用户头像 POST /upload/avatar
UPDATE users 
SET avatar_url = $2
WHERE user_id = $1;

-- =============================================
-- 3. 用户状态管理 (User Status Management)
-- =============================================

-- name: UpdateUserOnlineStatus :exec
-- 更新在线状态 PUT /users/me/status
UPDATE users 
SET online_status = $2
WHERE user_id = $1;

-- name: UpdateUserLastLogin :exec
-- 更新最后登录时间
UPDATE users 
SET last_login_at = NOW()
WHERE user_id = $1;

-- name: SetUserOffline :exec
-- 设置用户离线（退出登录时调用）
UPDATE users 
SET online_status = 'offline'
WHERE user_id = $1;

-- name: SetUserOnline :exec
-- 设置用户在线（登录时调用）
UPDATE users 
SET 
    online_status = 'online',
    last_login_at = NOW()
WHERE user_id = $1;

-- =============================================
-- 4. 用户搜索 (User Search)
-- =============================================

-- name: SearchUsers :many
-- 搜索用户 GET /users/search
SELECT 
    user_id,
    username,
    nickname,
    avatar_url,
    bio,
    online_status
FROM users 
WHERE 
    account_status = 'active'
    AND (
        username ILIKE '%' || $1 || '%' 
        OR nickname ILIKE '%' || $1 || '%'
    )
ORDER BY 
    CASE WHEN username = $1 THEN 0
         WHEN username ILIKE $1 || '%' THEN 1
         WHEN nickname = $1 THEN 2
         WHEN nickname ILIKE $1 || '%' THEN 3
         ELSE 4
    END,
    username
LIMIT $2 OFFSET $3;

-- name: CountSearchUsers :one
-- 搜索用户计数
SELECT COUNT(*) 
FROM users 
WHERE 
    account_status = 'active'
    AND (
        username ILIKE '%' || $1 || '%' 
        OR nickname ILIKE '%' || $1 || '%'
    );

-- name: GetUserPublicInfo :one
-- 获取用户公开信息（不含敏感信息）GET /users/:userId
SELECT 
    user_id,
    username,
    nickname,
    avatar_url,
    bio,
    online_status,
    registered_at
FROM users 
WHERE user_id = $1 AND account_status = 'active';

-- =============================================
-- 5. 账号管理 (Account Management)
-- =============================================

-- name: UpdateAccountStatus :exec
-- 更新账号状态（管理员操作）
UPDATE users 
SET account_status = $2
WHERE user_id = $1;

-- name: ActivateUser :exec
-- 激活用户账号
UPDATE users 
SET account_status = 'active'
WHERE user_id = $1 AND account_status = 'pending_verification';

-- name: SuspendUser :exec
-- 封禁用户账号（管理员操作）
UPDATE users 
SET account_status = 'suspended'
WHERE user_id = $1;

-- name: DeleteUserAccount :exec
-- 删除用户账号（软删除）
UPDATE users 
SET account_status = 'deleted'
WHERE user_id = $1;

-- name: GetUserSystemRole :one
-- 获取用户系统角色
SELECT system_role 
FROM users 
WHERE user_id = $1;

-- name: SetUserSystemRole :exec
-- 设置用户系统角色（超级管理员操作）
UPDATE users 
SET system_role = $2
WHERE user_id = $1;

-- name: IsUserAdmin :one
-- 检查用户是否为管理员
SELECT EXISTS(
    SELECT 1 FROM users 
    WHERE user_id = $1 AND system_role = 'admin'
) AS is_admin;

-- =============================================
-- 6. 批量查询 (Batch Queries)
-- =============================================

-- name: GetUsersByIDs :many
-- 批量获取用户信息
SELECT 
    user_id,
    username,
    nickname,
    avatar_url,
    bio,
    online_status
FROM users 
WHERE user_id = ANY($1::varchar[]) AND account_status = 'active';

-- name: GetOnlineUsers :many
-- 获取在线用户列表
SELECT 
    user_id,
    username,
    nickname,
    avatar_url,
    online_status
FROM users 
WHERE online_status IN ('online', 'away', 'do_not_disturb') AND account_status = 'active'
ORDER BY last_login_at DESC
LIMIT $1 OFFSET $2;

-- name: CountOnlineUsers :one
-- 统计在线用户数
SELECT COUNT(*) 
FROM users 
WHERE online_status IN ('online', 'away', 'do_not_disturb') AND account_status = 'active';