CREATE TABLE transfers (
    id              bigserial   PRIMARY KEY,
    from_account_id bigint      NOT NULL,
    to_account_id   bigint      NOT NULL,
    amount          bigint      NOT NULL CHECK (amount > 0),
    status          text        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'debited', 'completed', 'failed', 'refunded')),
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (from_account_id <> to_account_id)
);

-- A partial index: we only ever search for transfers that haven't finished.
CREATE INDEX transfers_unfinished_idx ON transfers (updated_at)
    WHERE status IN ('pending', 'debited');