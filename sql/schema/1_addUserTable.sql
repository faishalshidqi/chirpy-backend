-- +goose up
    create table users (
        id varchar primary key,
        created_at timestamp,
        updated_at timestamp,
        email text
    );

-- +goose down
drop table users;