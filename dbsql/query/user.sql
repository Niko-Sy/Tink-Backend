-- name: CreateUser :one
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

-- name: UpdateUser :one
UPDATE users 
SET 
    nickname = $2,
    phone_number = $3,
    email = $4,
    avatar_url = $5,
    bio = $6,
    online_status = $7,
    last_login_at = $8
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
UPDATE users 
SET hashed_password = $2
WHERE user_id = $1;