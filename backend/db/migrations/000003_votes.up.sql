-- 000003_votes.up.sql

CREATE TABLE votes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id    UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    -- NULL for anonymous votes (identity not recorded).
    user_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    answers    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Repeat-vote backstop: a plain UNIQUE treats NULLs as distinct, so any
    -- number of anonymous (user_id IS NULL) votes are allowed.
    UNIQUE (poll_id, user_id)
);

CREATE INDEX idx_votes_poll_id ON votes(poll_id);
CREATE INDEX idx_votes_user_id ON votes(user_id);
