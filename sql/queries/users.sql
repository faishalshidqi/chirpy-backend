-- name: CreateUser :one
insert into users (id, created_at, updated_at, email) values (gen_random_uuid(), NOW(), NOW(), $1) returning *;
-- name: EmptyUsersTable :exec
delete from users;

