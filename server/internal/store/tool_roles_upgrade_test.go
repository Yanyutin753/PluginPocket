package store

import "testing"

func TestToolAllowedRolesUpgrade(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		name := "existing_roles"
		if legacy {
			name = "original_031"
		}
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			if _, err := s.Pool.Exec(ctx, "DELETE FROM schema_migrations WHERE name > '031_billing_roles.sql'; INSERT INTO tools(key,name,kind,cost,allowed_roles) VALUES('upgrade','Preserved tool','http',7,'[\"vip\"]')"); err != nil {
				t.Fatal(err)
			}
			if legacy {
				if _, err := s.Pool.Exec(ctx, "ALTER TABLE tools DROP COLUMN allowed_roles"); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				if err := s.migrate(ctx); err != nil {
					t.Fatal(err)
				}
				var name, roles string
				var cost int64
				if err := s.Pool.QueryRow(ctx, "SELECT name,cost,allowed_roles::text FROM tools WHERE key='upgrade'").Scan(&name, &cost, &roles); err != nil {
					t.Fatalf("upgraded tools must be readable: %v", err)
				}
				want := `["vip"]`
				if legacy {
					want = `[]`
				}
				if name != "Preserved tool" || cost != 7 || roles != want {
					t.Fatalf("tool changed: %q %d %s", name, cost, roles)
				}
			}
		})
	}
}

func TestSQLiteToolAllowedRolesUpgradePreservesRestrictions(t *testing.T) {
	s := sqliteTestStore(t)
	ctx := t.Context()
	if _, err := s.DB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE name > '031_billing_roles.sql'; INSERT INTO tools(key,name,kind,cost,allowed_roles) VALUES('upgrade','Preserved tool','http',7,'[\"vip\"]')"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.migrate(ctx); err != nil {
			t.Fatal(err)
		}
		var roles string
		if err := s.DB.QueryRowContext(ctx, "SELECT allowed_roles FROM tools WHERE key='upgrade'").Scan(&roles); err != nil {
			t.Fatal(err)
		}
		if roles != `["vip"]` {
			t.Fatalf("restrictions changed: %s", roles)
		}
	}
}
