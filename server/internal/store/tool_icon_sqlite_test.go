package store

import "testing"

func TestSQLiteToolIcon(t *testing.T) {
	db := sqliteMigrate(t)
	var icon string
	if err := db.QueryRow("SELECT icon FROM tools LIMIT 1").Scan(&icon); err != nil {
		t.Fatal(err)
	}
	if icon != "" {
		t.Fatal(icon)
	}
	if _, err := db.Exec("UPDATE tools SET icon='https://example.com/icon.png'"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT icon FROM tools LIMIT 1").Scan(&icon); err != nil {
		t.Fatal(err)
	}
	if icon != "https://example.com/icon.png" {
		t.Fatal(icon)
	}
}
