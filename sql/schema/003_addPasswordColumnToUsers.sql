-- +goose Up
alter table users add hashed_password text not null;

-- +goose Down
alter table users drop column hashed_password;