-- Phase 4 full ingestion: canonical transaction size/weight/vsize/feerate,
-- exact satoshi amounts, script-type provenance, and record completeness.
-- Columns are added (not rewritten) so Phase 0-3 case databases still open.

ALTER TABLE transactions ADD COLUMN fee_sats       INTEGER DEFAULT 0;
ALTER TABLE transactions ADD COLUMN base_size      INTEGER DEFAULT 0;
ALTER TABLE transactions ADD COLUMN total_size     INTEGER DEFAULT 0;
ALTER TABLE transactions ADD COLUMN weight_wu      INTEGER DEFAULT 0;
ALTER TABLE transactions ADD COLUMN vsize_vb       INTEGER DEFAULT 0;
ALTER TABLE transactions ADD COLUMN feerate_sat_vb REAL    DEFAULT 0;
ALTER TABLE transactions ADD COLUMN orig_script_type TEXT;
ALTER TABLE transactions ADD COLUMN completeness   TEXT DEFAULT 'valid';
ALTER TABLE transactions ADD COLUMN total_in_sats  INTEGER DEFAULT 0;
ALTER TABLE transactions ADD COLUMN total_out_sats INTEGER DEFAULT 0;

ALTER TABLE transaction_inputs  ADD COLUMN amount_sats INTEGER DEFAULT 0;
ALTER TABLE transaction_outputs ADD COLUMN amount_sats INTEGER DEFAULT 0;
