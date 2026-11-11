CREATE TABLE accounts (
    id         bigserial   PRIMARY KEY,
    owner      text        NOT NULL,
    balance    bigint      NOT NULL DEFAULT 0,
    currency   char(3)     NOT NULL DEFAULT 'USD',
    created_at timestamptz NOT NULL DEFAULT now()
);