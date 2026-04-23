-- Replace single-column indexes with composite indexes for cursor-based pagination.
-- Composite indexes serve both filter-by-account_id and filter-by-account_id+cursor-on-id.
DROP INDEX IF EXISTS idx_transfers_from_account_id;
DROP INDEX IF EXISTS idx_transfers_to_account_id;
CREATE INDEX idx_transfers_from_account_id ON transfers(from_account_id, id DESC);
CREATE INDEX idx_transfers_to_account_id ON transfers(to_account_id, id DESC);
