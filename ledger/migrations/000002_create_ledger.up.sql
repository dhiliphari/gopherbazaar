ALTER TABLE accounts ADD CONSTRAINT accounts_balance_non_negative CHECK (balance >= 0);

CREATE TABLE transfers (
    id              bigserial   PRIMARY KEY,
    from_account_id bigint      NOT NULL REFERENCES accounts (id),
    to_account_id   bigint      NOT NULL REFERENCES accounts (id),
    amount          bigint      NOT NULL CHECK (amount > 0),
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (from_account_id <> to_account_id)
);

CREATE TABLE entries (
    id          bigserial   PRIMARY KEY,
    account_id  bigint      NOT NULL REFERENCES accounts (id),
    transfer_id bigint      REFERENCES transfers (id),   -- NULL for deposits
    amount      bigint      NOT NULL CHECK (amount <> 0),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX entries_account_id_idx ON entries (account_id);

INSERT INTO entries (account_id, amount)
SELECT id, balance FROM accounts WHERE balance > 0;