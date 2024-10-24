// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: users.sql

package database

import (
	"context"
	"database/sql"
)

const createUser = `-- name: CreateUser :one
insert into users (id, created_at, updated_at, email) values (gen_random_uuid(), NOW(), NOW(), $1) returning id, created_at, updated_at, email
`

func (q *Queries) CreateUser(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRowContext(ctx, createUser, email)
	var i User
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Email,
	)
	return i, err
}

const emptyUsersTable = `-- name: EmptyUsersTable :execresult
delete from users
`

func (q *Queries) EmptyUsersTable(ctx context.Context) (sql.Result, error) {
	return q.db.ExecContext(ctx, emptyUsersTable)
}
