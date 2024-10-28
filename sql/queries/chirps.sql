-- name: CreateChirp :one
insert into chirps (id, created_at, updated_at, body, user_id) values (gen_random_uuid(), NOW(), NOW(), $1, $2) returning *;
-- name: RetrieveChirps :many
select * from chirps order by created_at asc;
-- name: RetrieveChirpById :one
select * from chirps where id = $1;
-- name: RetrieveChirpsByAuthor :many
select * from chirps where user_id = $1 order by created_at asc;
-- name: DeleteChirpById :exec
delete from chirps where id = $1 returning *;