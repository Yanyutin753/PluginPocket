package store

import "testing"

func TestSQLiteBrowserSessionRevocation(t *testing.T) {
	db := sqliteMigrate(t)
	if _, err := db.Exec(`INSERT INTO users(username,password_hash) VALUES('browser','hash'); INSERT INTO sessions(user_id,session_hash,expires_at) VALUES(1,'refresh','2030-01-01'); INSERT INTO session_access(access_hash,session_id,expires_at) VALUES('access',1,'2029-01-01')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM sessions WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM session_access").Scan(&count); err != nil || count != 0 {
		t.Fatalf("revocation did not cascade: count=%d err=%v", count, err)
	}
}
