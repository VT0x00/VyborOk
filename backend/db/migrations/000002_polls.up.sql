-- 000002_polls.up.sql

CREATE TABLE polls (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        VARCHAR(255) NOT NULL,
    description  TEXT,
    status       VARCHAR(20)  NOT NULL DEFAULT 'active'
                 CHECK (status IN ('active', 'closed')),
    anonymous    BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_polls_user_id_created ON polls(user_id, created_at DESC);
CREATE INDEX idx_polls_status_created ON polls(status, created_at DESC);

CREATE TABLE questions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id         UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    type            VARCHAR(20) NOT NULL
                    CHECK (type IN ('single', 'multiple', 'scale', 'text')),
    title           TEXT        NOT NULL,
    description     TEXT,
    position        INT         NOT NULL,
    -- Специфично для type='scale':
    scale_min       INT,
    scale_max       INT,
    -- Специфично для type='text':
    text_max_length INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (poll_id, position)
);

CREATE TABLE options (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    text        TEXT        NOT NULL,
    position    INT         NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (question_id, position)
);
