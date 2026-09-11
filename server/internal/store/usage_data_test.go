package store

import (
	"sync"
	"testing"
)

func TestFinishWithDataConcurrentRefundAndRollback(t *testing.T) {
	s := testStore(t)
	user, wallet, token := fixture(t, s)
	ctx := t.Context()
	call, err := s.Reserve(ctx, user, token, wallet, "echo", 2, "payload-refund")
	if err != nil {
		t.Fatal(err)
	}
	input, output := "original input", "original output"
	data := &CallData{InputData: &input, OutputData: &output}
	// A failed refund must roll back the payload too, leaving recovery possible.
	if _, err := s.Pool.Exec(ctx, `CREATE FUNCTION reject_payload_refund() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind='refund' THEN RAISE EXCEPTION 'test refund failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_payload_refund BEFORE INSERT ON ledger FOR EACH ROW EXECUTE FUNCTION reject_payload_refund()`); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishWithData(ctx, call.ID, false, 0, data); err == nil {
		t.Fatal("failed refund must not commit")
	}
	var saved *string
	var status string
	if err := s.Pool.QueryRow(ctx, "SELECT input_data,status FROM usage_logs WHERE id=$1", call.ID).Scan(&saved, &status); err != nil {
		t.Fatal(err)
	}
	if saved != nil || status != "pending" {
		t.Fatal("failed settlement partially committed payload")
	}
	if _, err := s.Pool.Exec(ctx, "DROP TRIGGER reject_payload_refund ON ledger"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if err := s.FinishWithData(ctx, call.ID, false, 0, data); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	var balance, refunds int64
	if err := s.Pool.QueryRow(ctx, `SELECT balance,(SELECT count(*) FROM ledger WHERE call_id=$2 AND kind='refund') FROM wallets WHERE id=$1`, wallet, call.ID).Scan(&balance, &refunds); err != nil {
		t.Fatal(err)
	}
	if balance != 10 || refunds != 1 {
		t.Fatalf("balance=%d refunds=%d", balance, refunds)
	}
	overwrite := "late result"
	if err := s.FinishWithData(ctx, call.ID, true, 0, &CallData{InputData: &overwrite}); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "SELECT input_data,status FROM usage_logs WHERE id=$1", call.ID).Scan(&saved, &status); err != nil {
		t.Fatal(err)
	}
	if saved == nil || *saved != input || status != "error" {
		t.Fatal("late completion overwrote settled record")
	}
}
