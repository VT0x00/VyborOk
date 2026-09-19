-- 000001_init.up.sql

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    username        VARCHAR(50)  NOT NULL UNIQUE,
    first_name      VARCHAR(100),
    last_name       VARCHAR(100),
    bio             TEXT,
    links           JSONB        NOT NULL DEFAULT '[]'::jsonb,
    avatar_url      VARCHAR(255),
    is_private      BOOLEAN      NOT NULL DEFAULT FALSE,
    public_fields   JSONB        NOT NULL DEFAULT '[]'::jsonb,
    email_verified  BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Индексы для частых запросов.
CREATE INDEX idx_users_email    ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_is_private ON users(is_private);
CREATE INDEX idx_users_created_at ON users(created_at);
