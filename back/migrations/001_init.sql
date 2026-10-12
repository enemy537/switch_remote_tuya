create extension if not exists pgcrypto;

create table users (
    id            bigint generated always as identity primary key,
    email         text        not null unique,
    password_hash text        not null,
    is_admin      boolean     not null default false,
    tuya_uid      text        not null default '',
    created_at    timestamptz not null default now()
);

create table devices (
    id         text        primary key, -- Tuya device id
    user_id    bigint      not null references users on delete cascade,
    name       text        not null,
    created_at timestamptz not null default now()
);

create index devices_user_id_idx on devices (user_id);

-- Seed the first admin (email must be lowercase, password at least 8 chars):
-- insert into users (email, password_hash, is_admin) values ('admin@example.com', crypt('PASSWORD', gen_salt('bf', 10)), true);
