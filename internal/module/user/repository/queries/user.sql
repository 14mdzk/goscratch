-- name: GetUserByID :one
SELECT id, email, password_hash, name, is_active, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, name, is_active, created_at, updated_at
FROM users
WHERE email = $1 AND is_active = true;

-- name: ListUsers :many
SELECT id, email, password_hash, name, is_active, created_at, updated_at
FROM users
WHERE (sqlc.narg(cursor)::uuid IS NULL OR id > sqlc.narg(cursor))
  AND (sqlc.narg(search)::text IS NULL OR name ILIKE '%' || sqlc.narg(search) || '%' OR email ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(email_filter)::text IS NULL OR email = sqlc.narg(email_filter))
  AND (sqlc.narg(is_active)::bool IS NULL OR is_active = sqlc.narg(is_active))
ORDER BY id ASC
LIMIT $1;

-- name: ListUsersPrev :many
SELECT id, email, password_hash, name, is_active, created_at, updated_at
FROM users
WHERE (sqlc.narg(cursor)::uuid IS NULL OR id < sqlc.narg(cursor))
  AND (sqlc.narg(search)::text IS NULL OR name ILIKE '%' || sqlc.narg(search) || '%' OR email ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(email_filter)::text IS NULL OR email = sqlc.narg(email_filter))
  AND (sqlc.narg(is_active)::bool IS NULL OR is_active = sqlc.narg(is_active))
ORDER BY id DESC
LIMIT $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, name, is_active)
VALUES ($1, $2, $3, true)
RETURNING id, email, password_hash, name, is_active, created_at, updated_at;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE(NULLIF($2, ''), name),
    email = COALESCE(NULLIF($3, ''), email),
    updated_at = NOW()
WHERE id = $1
RETURNING id, email, password_hash, name, is_active, created_at, updated_at;

-- name: UpdatePassword :exec
UPDATE users
SET password_hash = $2, updated_at = NOW()
WHERE id = $1 AND is_active = true;

-- name: DeleteUser :exec
UPDATE users
SET is_active = false, updated_at = NOW()
WHERE id = $1;

-- name: ActivateUser :exec
UPDATE users
SET is_active = true, updated_at = NOW()
WHERE id = $1;

-- name: DeactivateUser :exec
UPDATE users
SET is_active = false, updated_at = NOW()
WHERE id = $1;

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE (sqlc.narg(is_active)::bool IS NULL OR is_active = sqlc.narg(is_active));

-- name: UserExistsByEmail :one
SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND is_active = true);
