CREATE TABLE accounts (
    id         bigserial   PRIMARY KEY,
    owner      text        NOT NULL,
    balance    bigint      NOT NULL DEFAULT 0 CHECK (balance >= 0),
    currency   char(3)     NOT NULL DEFAULT 'USD',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE entries (
    id          bigserial   PRIMARY KEY,
    account_id  bigint      NOT NULL REFERENCES accounts (id),
    -- transfer_id refers to a row in the *transfers* service's database
    transfer_id bigint,
    kind        text        NOT NULL CHECK (kind IN ('deposit', 'debit', 'credit', 'refund')),
    amount      bigint      NOT NULL CHECK (amount <> 0),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX entries_account_id_idx ON entries (account_id);