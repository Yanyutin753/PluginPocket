-- A caller-controlled adjustment key cannot collide with a redemption/transfer key.
ALTER TABLE ledger DROP CONSTRAINT ledger_wallet_id_idempotency_key_key;
ALTER TABLE ledger ADD CONSTRAINT ledger_operation_idempotency_key
 UNIQUE(wallet_id,kind,idempotency_key);
