-- name: CreateUser :one
insert into users (id, created_at, updated_at, email, hashed_password) values (gen_random_uuid(), NOW(), NOW(), $1, $2) returning *;
-- name: EmptyUsersTable :exec
delete from users;
-- name: GetUserByEmail :one
select * from users where email = $1;