DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS transfers;
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_balance_non_negative;