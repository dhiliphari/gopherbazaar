-- Each transfer may debit, credit or refund a given account at most once.
-- The store checks this before writing; this index makes it impossible to
-- get wrong, even if a future code change forgets the check.
CREATE UNIQUE INDEX entries_once_per_transfer_idx
    ON entries (transfer_id, account_id, kind)
    WHERE transfer_id IS NOT NULL;