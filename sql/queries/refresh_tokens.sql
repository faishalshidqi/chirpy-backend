-- name: CreateRefreshToken :one
insert into refresh_tokens (token, created_at, updated_at, user_id, expires_at, revoked_at) values ($1, NOW(), NOW(), $2, $3, $4) returning *;
-- name: GetRefreshTokenByToken :one
select * from refresh_tokens where token = $1;
-- name: UpdateRefreshTokenByToken :exec
update refresh_tokens set token = $1, created_at = $2, updated_at = NOW(), user_id = $3, expires_at = $4, revoked_at = $5 where user_id = $3;
-- name: DeleteRefreshTokenByToken :exec
delete from refresh_tokens where token = $1;