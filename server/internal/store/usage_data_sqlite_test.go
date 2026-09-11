package store

import "testing"

func TestSQLiteUsageDataRoundTrip(t *testing.T) {
	db := sqliteMigrate(t)
	for _, q := range []string{
		"INSERT INTO users(id,username,password_hash) VALUES(1,'payload','h')",
		"INSERT INTO wallets(id,user_id) VALUES(1,1)",
		"INSERT INTO tokens(id,user_id,wallet_id,name,prefix,token_hash) VALUES(1,1,1,'test','ppt_','hash')",
		"INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,request_key,status) VALUES(1,1,1,'echo','payload','ok')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	var input, output *string
	var it, ot bool
	if err := db.QueryRow("SELECT input_data,output_data,input_truncated,output_truncated FROM usage_logs").Scan(&input, &output, &it, &ot); err != nil {
		t.Fatal(err)
	}
	if input != nil || output != nil || it || ot {
		t.Fatal("legacy defaults must be unrecorded")
	}
	if _, err := db.Exec("UPDATE usage_logs SET input_data=?,output_data=?,output_truncated=1", `{"token":"original"}`, "partial"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT input_data,output_data,input_truncated,output_truncated FROM usage_logs").Scan(&input, &output, &it, &ot); err != nil {
		t.Fatal(err)
	}
	if input == nil || *input != `{"token":"original"}` || output == nil || *output != "partial" || it || !ot {
		t.Fatal("payload not preserved")
	}
}
