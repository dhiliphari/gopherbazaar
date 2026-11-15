ALTER TABLE transfers DROP COLUMN IF EXISTS locked_until;
ALTER TABLE transfers DROP COLUMN IF EXISTS idempotency_key;