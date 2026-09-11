package app

import (
	"encoding/json"
	"testing"
)

func TestAnonymousPluginAPI(t *testing.T) {
	f := marketplaceApp(t, nil)
	for _, path := range []string{"/api/v1/plugins", "/api/v1/plugins/deepwiki"} {
		w := request(f.handler, "GET", path, "", nil)
		if w.Code != 200 {
			t.Fatalf("anonymous %s: %d", path, w.Code)
		}
		if w.Header().Get("Content-Type") != "application/json" {
			t.Fatal("public API must return JSON")
		}
		var data map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if len(data["origin"]) == 0 {
			t.Fatal("missing shared public origin")
		}
		var items []map[string]json.RawMessage
		if path == "/api/v1/plugins" {
			if err := json.Unmarshal(data["items"], &items); err != nil {
				t.Fatal(err)
			}
		} else {
			var item map[string]json.RawMessage
			if err := json.Unmarshal(data["item"], &item); err != nil {
				t.Fatal(err)
			}
			items = append(items, item)
		}
		if len(items) == 0 {
			t.Fatal("anonymous directory lost seed plugins")
		}
		for _, item := range items {
			for key := range item {
				switch key {
				case "slug", "name", "description", "kind", "version", "gateway":
				default:
					t.Fatalf("non-public field exposed: %s", key)
				}
			}
		}
	}
	if w := request(f.handler, "GET", "/api/v1/plugins/missing", "", nil); w.Code != 404 {
		t.Fatalf("missing plugin: %d", w.Code)
	}
}
