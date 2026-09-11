package store

import (
	"context"
	"strings"
	"testing"
)

func TestPostgresUpgradeOriginalFileLimit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.Pool.Exec(ctx, "ALTER TABLE file_objects DROP CONSTRAINT file_objects_size_check; ALTER TABLE file_objects ADD CONSTRAINT file_objects_size_check CHECK (size BETWEEN 0 AND 8388608)"); err != nil {
		t.Fatal(err)
	}
	raw, err := migrations.ReadFile("migrations/030_file_object_limit.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, "INSERT INTO file_objects(sha256,size,content) VALUES($1,8388609,$2)", strings.Repeat("b", 64), make([]byte, 8388609)); err != nil {
		t.Fatalf("old 8MiB limit not upgraded: %v", err)
	}
}
