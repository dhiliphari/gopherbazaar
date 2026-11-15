-- A client-supplied key that identifies one logical transfer request.
-- UNIQUE ignores NULLs, so requests without a key are unaffected.
ALTER TABLE transfers ADD COLUMN idempotency_key text UNIQUE;

-- While a resumer is working on a transfer, it holds a lease until this time.
ALTER TABLE transfers ADD COLUMN locked_until timestamptz;