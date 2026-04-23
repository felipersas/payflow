-- Revert to single-column indexes.
DROP INDEX IF EXISTS idx_transfers_from_account_id;
DROP INDEX IF EXISTS idx_transfers_to_account_id;
CREATE INDEX idx_transfers_from_account_id ON transfers(from_account_id);
CREATE INDEX idx_transfers_to_account_id ON transfers(to_account_id);
