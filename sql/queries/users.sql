-- name: CreateUser :one
insert into users (id, created_at, updated_at, email, hashed_password) values (gen_random_uuid(), NOW(), NOW(), $1, $2) returning *;
-- name: EmptyUsersTable :exec
delete from users;
-- name: GetUserByEmail :one
select * from users where email = $1;
-- name: UpdateUserByID :one
update users set id = $1, created_at = $2, updated_at = NOW(), email = $3, hashed_password = $4, is_chirpy_red = $5 where id = $1 returning *;
-- name: GetUserById :one
select * from users where id = $1;